import { stringify } from 'yaml';
import { composeLabels, dockerfileLabelBlock } from './labels.js';
import { composeServices, getComposeService } from './registry.js';
import { resolveDockerfileModules } from './resolver.js';
import {
  GENERATED_HEADER,
  GENERATED_HEADER_YAML,
  normalizeServices,
  type DevcontainerConfig,
} from './types.js';

export function generateDockerfile(config: DevcontainerConfig): string {
  const resolved = resolveDockerfileModules(config.dockerfile.modules);
  const fragments = resolved.map((r) => r.module.render(r.options).trim());
  fragments.push(dockerfileLabelBlock(config));
  return `${GENERATED_HEADER}\n\n${fragments.join('\n\n')}\n`;
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

export function generateCompose(config: DevcontainerConfig): string {
  const selected = normalizeServices(config.compose.services);
  const optionsById = new Map<string, Record<string, unknown>>();
  const enabled = new Set<string>();
  for (const s of selected) {
    enabled.add(s.id);
    optionsById.set(s.id, s.options ?? {});
  }
  for (const svc of composeServices) {
    if (svc.always) enabled.add(svc.id);
  }
  const enabledIds = [...enabled];

  const workspace = config.workspace;
  const networkName = `${workspace}-network`;
  const labels = composeLabels(config);
  const services: Record<string, unknown> = {};
  const volumes: Record<string, unknown> = {};

  const declaredVolumes = new Set<string>(BASE_VOLUMES);
  for (const id of enabledIds) {
    const svc = getComposeService(id);
    if (!svc) throw new Error(`Unknown compose service: ${id}`);
    for (const v of svc.volumes ?? []) declaredVolumes.add(v);
  }

  for (const id of enabledIds) {
    const svc = getComposeService(id);
    if (!svc) throw new Error(`Unknown compose service: ${id}`);
    const rendered = svc.render({
      imageName: config.image,
      enabledServiceIds: enabledIds,
      options: optionsById.get(id) ?? {},
    }) as Record<string, unknown>;

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
      rendered.networks = (rendered.networks as string[]).map((n) =>
        n === 'local-network' ? networkName : n,
      );
    }
    rendered.labels = { ...labels };
    services[svc.id === 'devcontainer' ? 'devcontainer-ssh' : svc.id] = rendered;
  }

  for (const v of declaredVolumes) {
    volumes[prefixVolume(workspace, v)] = { labels: { ...labels } };
  }

  const subnet = config.compose.subnet ?? '172.25.0.0/24';
  const doc: Record<string, unknown> = {
    services,
    networks: {
      [networkName]: {
        driver: 'bridge',
        ipam: { config: [{ subnet: `\${DOCKER_SUBNET:-${subnet}}` }] },
        labels: { ...labels },
      },
    },
    volumes,
  };

  return `${GENERATED_HEADER_YAML}\n${stringify(doc, { lineWidth: 0 })}`;
}

export function plannedComposeNames(config: DevcontainerConfig): {
  containers: string[];
  network: string;
  volumes: string[];
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
  const enabledIds = [...enabled];

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
  const lines: string[] = ['# Generated by devcontainer CLI'];
  for (const [k, v] of Object.entries(config.env)) {
    lines.push(`${k}=${v}`);
  }
  if (!('DOCKER_SUBNET' in config.env) && config.compose.subnet) {
    lines.push(`DOCKER_SUBNET=${config.compose.subnet}`);
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

export function collectRequiredEnvVars(
  config: DevcontainerConfig,
): { name: string; prompt: string; default?: string }[] {
  const selected = normalizeServices(config.compose.services);
  const out: { name: string; prompt: string; default?: string }[] = [];
  for (const s of selected) {
    const svc = getComposeService(s.id);
    for (const e of svc?.requiresEnv ?? []) out.push(e);
  }
  return out;
}
