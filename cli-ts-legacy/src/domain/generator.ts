import { stringify } from 'yaml';
import { composeLabels, dockerfileLabelBlock } from '@/core/labels.js';
import { composeServices, getComposeService } from '@/core/module-registry.js';
import { DIND_ENGINE_HOST } from '@/modules/compose/dind-engine.js';
import type { DockerSocketMode } from '@/modules/compose/devcontainer.js';
import { resolveDockerfileModules } from '@/domain/resolver.js';
import { SSH_DEFAULTS } from '@/infra/ssh-defaults.js';
import { lastHost } from '@/domain/subnet.js';
import { resolveRegistry } from '@/domain/global-config.js';
import {
  GENERATED_HEADER,
  POST_SCRIPT_DIR,
  normalizeServices,
  type DevcontainerConfig,
  type RemoteVariant,
  type RequiredEnvVar,
} from '@/core/types.js';

const DEFAULT_SUBNET = '172.25.0.0/28';

// Resolves the full set of enabled compose services for a config: the
// user-selected ones, the always-on ones, plus services that are
// auto-provisioned from another setting (the rootless dind engine is pulled
// in by the devcontainer's `dind` access mode, never selected directly).
function resolveEnabledServices(config: DevcontainerConfig): {
  enabledIds: string[];
  optionsById: Map<string, Record<string, unknown>>;
} {
  const selected = normalizeServices(config.compose.services);
  const enabled = new Set<string>();
  const optionsById = new Map<string, Record<string, unknown>>();
  for (const s of selected) {
    enabled.add(s.id);
    optionsById.set(s.id, s.options ?? {});
  }
  for (const svc of composeServices) {
    if (svc.always) enabled.add(svc.id);
  }
  const devOpts = optionsById.get('devcontainer');
  if ((devOpts?.dockerSocket as DockerSocketMode | undefined) === 'dind') {
    enabled.add(DIND_ENGINE_HOST);
  }
  return { enabledIds: [...enabled], optionsById };
}

export function generateDockerfile(config: DevcontainerConfig): string | null {
  if (config.mode === 'remote') return null;
  const resolved = resolveDockerfileModules(config.dockerfile.modules);
  const fragments = resolved.map((r) => r.module.render(r.options).trim());
  const postScripts = postScriptsDockerfileBlock(config);
  if (postScripts) fragments.push(postScripts);
  fragments.push(dockerfileLabelBlock(config));
  return `${GENERATED_HEADER}\n\n${fragments.join('\n\n')}\n`;
}

// Bakes every declared post-install script into POST_SCRIPT_DIR. Driven entirely by
// each module's `postScriptFiles` declaration — adding a new script needs no change
// here: declare it on a module and drop the file in assets/.
function postScriptsDockerfileBlock(config: DevcontainerConfig): string {
  const files = collectRequiredPostScriptFiles(config);
  if (files.length === 0) return '';
  return `##
## POST-INSTALL SCRIPTS (available inside the container, not auto-run)
##
COPY ${files.join(' ')} ${POST_SCRIPT_DIR}/
RUN chown -R devuser:devuser ${POST_SCRIPT_DIR} && chmod +x ${POST_SCRIPT_DIR}/*.sh`;
}

export function resolveRemoteImage(
  variant: RemoteVariant,
  registryOverride?: string,
  perProject?: string,
): string {
  const registry = resolveRegistry(registryOverride, perProject);
  const suffix = variant === 'ssh' ? 'devcontainer-ssh' : `devcontainer-${variant}`;
  return `${registry}${suffix}:latest`;
}

export function resolveDevcontainerImageName(config: DevcontainerConfig): string {
  if (config.mode === 'remote' && config.remote) {
    return resolveRemoteImage(config.remote.variant, undefined, config.remote.registry);
  }
  if (config.mode === 'local-cached' && config.fingerprint) {
    return `devcontainer-cli/${config.fingerprint.slice(0, 12)}:latest`;
  }
  return config.image;
}

const BASE_VOLUMES = ['devcontainer_etc', 'devcontainer_root', 'devcontainer_home'];

function prefixContainer(workspace: string, base: string): string {
  if (base === workspace || base.startsWith(`${workspace}-`)) return base;
  return `${workspace}-${base}`;
}

function prefixVolume(workspace: string, base: string): string {
  if (base.startsWith(`${workspace}_`)) return base;
  return `${workspace}_${base}`;
}

function remapVolumeMount(workspace: string, declared: Set<string>, mount: string): string {
  const colon = mount.indexOf(':');
  if (colon <= 0) return mount;
  const src = mount.slice(0, colon);
  if (!declared.has(src)) return mount;
  return `${prefixVolume(workspace, src)}${mount.slice(colon)}`;
}

export function generateCompose(config: DevcontainerConfig): string | null {
  const { enabledIds, optionsById } = resolveEnabledServices(config);

  const workspace = config.workspace;
  const networkName = `${workspace}-network`;
  const engineNetwork = `${workspace}-engine-network`;
  let usesEngineNetwork = false;
  const mapNetwork = (n: string): string => {
    if (n === 'local-network') return networkName;
    if (n === 'engine-network') {
      usesEngineNetwork = true;
      return engineNetwork;
    }
    return n;
  };
  const labels = composeLabels(config);
  const services: Record<string, unknown> = {};
  const volumes: Record<string, unknown> = {};
  const subnet = config.compose.subnet ?? DEFAULT_SUBNET;
  const devcontainerIp = lastHost(subnet);

  const declaredVolumes = new Set<string>(BASE_VOLUMES);
  for (const id of enabledIds) {
    const svc = getComposeService(id);
    if (!svc) throw new Error(`Unknown compose service: ${id}`);
    for (const v of svc.volumes ?? []) declaredVolumes.add(v);
  }

  const devcontainerImageName = resolveDevcontainerImageName(config);

  for (const id of enabledIds) {
    const svc = getComposeService(id);
    if (!svc) throw new Error(`Unknown compose service: ${id}`);
    const imageNameForSvc = svc.id === 'devcontainer' ? devcontainerImageName : config.image;
    const rendered = svc.render({
      imageName: imageNameForSvc,
      enabledServiceIds: enabledIds,
      options: optionsById.get(id) ?? {},
    }) as Record<string, unknown>;

    if (svc.id === 'devcontainer' && config.mode === 'remote') {
      delete rendered.build;
    }

    const baseContainer =
      typeof rendered.container_name === 'string'
        ? (rendered.container_name as string)
        : svc.id;
    rendered.container_name = prefixContainer(workspace, baseContainer);

    if (Array.isArray(rendered.volumes)) {
      rendered.volumes = (rendered.volumes as string[]).map((m) =>
        remapVolumeMount(workspace, declaredVolumes, m),
      );
    }
    if (Array.isArray(rendered.networks)) {
      const mapped = (rendered.networks as string[]).map(mapNetwork);
      if (svc.id === 'devcontainer' && devcontainerIp) {
        const netObj: Record<string, unknown> = {};
        for (const n of mapped) {
          netObj[n] = n === networkName
            ? { ipv4_address: `\${DEVCONTAINER_IP:-${devcontainerIp}}` }
            : null;
        }
        rendered.networks = netObj;
      } else {
        rendered.networks = mapped;
      }
    }
    rendered.labels = { ...labels };
    services[svc.id === 'devcontainer' ? SSH_DEFAULTS.serviceName : svc.id] = rendered;
  }

  for (const v of declaredVolumes) {
    volumes[prefixVolume(workspace, v)] = { labels: { ...labels } };
  }

  const networksDoc: Record<string, unknown> = {
    [networkName]: {
      driver: 'bridge',
      ipam: { config: [{ subnet: `\${DOCKER_SUBNET:-${subnet}}` }] },
      labels: { ...labels },
    },
  };
  if (usesEngineNetwork) {
    // Dedicated bridge joined only by the devcontainer and the dind engine.
    // Not `internal` — the engine needs egress to pull workload images; the
    // isolation here is segmentation (DB/other sidecars stay off this network).
    networksDoc[engineNetwork] = {
      driver: 'bridge',
      labels: { ...labels },
    };
  }

  const doc: Record<string, unknown> = {
    services,
    networks: networksDoc,
    volumes,
  };

  return `${GENERATED_HEADER}\n${stringify(doc, { lineWidth: 0 })}`;
}

export function plannedComposeNames(config: DevcontainerConfig): {
  containers: string[];
  network: string;
  volumes: string[];
} {
  const { enabledIds, optionsById } = resolveEnabledServices(config);

  const containers: string[] = [];
  const declaredVolumes = new Set<string>(BASE_VOLUMES);
  for (const id of enabledIds) {
    const svc = getComposeService(id);
    if (!svc) continue;
    for (const v of svc.volumes ?? []) declaredVolumes.add(v);
    const rendered = svc.render({
      imageName: config.image,
      enabledServiceIds: enabledIds,
      options: optionsById.get(id) ?? {},
    }) as Record<string, unknown>;
    const base =
      typeof rendered.container_name === 'string'
        ? (rendered.container_name as string)
        : svc.id;
    containers.push(prefixContainer(config.workspace, base));
  }

  return {
    containers,
    network: `${config.workspace}-network`,
    volumes: [...declaredVolumes].map((v) => prefixVolume(config.workspace, v)),
  };
}

export function generateEnv(config: DevcontainerConfig): string {
  const env: Record<string, string> = { ...config.env };
  if (!('DOCKER_SUBNET' in env) && config.compose.subnet) {
    env.DOCKER_SUBNET = config.compose.subnet;
  }
  // Derive DEVCONTAINER_IP from the subnet that will actually be written, so
  // the two stay in sync even when DOCKER_SUBNET comes from config.env.
  if (!('DEVCONTAINER_IP' in env)) {
    const effectiveSubnet = env.DOCKER_SUBNET ?? config.compose.subnet ?? DEFAULT_SUBNET;
    const ip = lastHost(effectiveSubnet);
    if (ip) env.DEVCONTAINER_IP = ip;
  }
  const lines: string[] = ['# Generated by devcontainer CLI'];
  for (const [k, v] of Object.entries(env)) {
    lines.push(`${k}=${v}`);
  }
  return lines.join('\n') + '\n';
}

export function collectRequiredCopyFiles(config: DevcontainerConfig): string[] {
  const resolved = resolveDockerfileModules(config.dockerfile.modules);
  const files = new Set<string>();
  for (const r of resolved) {
    for (const f of r.module.copyFiles ?? []) files.add(f);
  }
  return [...files];
}

export function collectRequiredPostScriptFiles(config: DevcontainerConfig): string[] {
  const resolved = resolveDockerfileModules(config.dockerfile.modules);
  const files = new Set<string>();
  for (const r of resolved) {
    const ps = r.module.postScriptFiles;
    const list = typeof ps === 'function' ? ps(r.options) : (ps ?? []);
    for (const f of list) files.add(f);
  }
  return [...files];
}

export function collectRequiredEnvVars(
  config: DevcontainerConfig,
): RequiredEnvVar[] {
  const selected = normalizeServices(config.compose.services);
  const out: RequiredEnvVar[] = [];
  for (const s of selected) {
    const svc = getComposeService(s.id);
    for (const e of svc?.requiresEnv ?? []) out.push(e);
  }
  return out;
}
