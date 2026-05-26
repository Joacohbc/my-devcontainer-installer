import * as fs from 'fs';
import * as path from 'path';
import chalk from 'chalk';

import { parseFlags, helpText, type CliFlags } from '@/cli.js';
import { defaultConfig, loadConfig, saveConfig } from '@/domain/config.js';
import {
  generateDockerfile,
  generateCompose,
  generateEnv,
  collectRequiredCopyFiles,
  collectRequiredEnvVars,
  collectRequiredPostScriptFiles,
  resolveRemoteImage,
} from '@/domain/generator.js';
import { findConflicts } from '@/domain/docker-conflicts.js';
import { isGeneratedFile, preflight, validateRequiredFiles } from '@/infra/preflight.js';
import { findFreeSubnet, formatCidr, listUsedSubnets, subnetConflict } from '@/domain/subnet.js';
import { printSshInstructions } from '@/infra/ssh-instructions.js';
import { confirm, input, multiselect, select } from '@/infra/prompts.js';
import { composeServices, dockerfileModules, getDockerfileModule } from '@/core/module-registry.js';
import { HOST_DOCKER_SOCKET, type DockerSocketMode } from '@/modules/compose/devcontainer.js';
import { getCurrentVersion } from '@/commands/self-update.js';
import {
  computeFingerprint,
  fingerprintTag,
  localImageExists,
  recordProject,
} from '@/domain/image-registry.js';
import {
  BUILD_MODES,
  REMOTE_VARIANTS,
  VARIANT_LABELS,
  type BuildMode,
  type DevcontainerConfig,
  type ModuleOption,
  type RemoteVariant,
  type SelectedModule,
} from '@/core/types.js';
import { isValidCidr, isValidDockerName, isValidImageName, sanitizeDockerName } from '@/domain/validators.js';
import { projectPaths, resolveWorkspace } from '@/infra/project.js';
import { dockerCompose, dockerInherit } from '@/infra/docker.js';

const MODE_LABELS: Record<BuildMode, string> = {
  'local-cached': 'local-cached — generate Dockerfile + compose, reuse cached image when unchanged',
  remote: 'remote — skip the build, pull a pre-built image from the registry',
};

async function buildConfigFromPrompts(base: DevcontainerConfig): Promise<DevcontainerConfig> {
  const workspace = await input(
    'workspace',
    'Workspace name (used as prefix for containers, network, volumes):',
    base.workspace || sanitizeDockerName(path.basename(process.cwd())),
    (v) => (isValidDockerName(v) ? true : 'Invalid Docker name (allowed: a-z A-Z 0-9 _ . -, ≤63 chars, start alphanumeric)'),
  );
  base.workspace = workspace;

  const mode = (await select(
    'mode',
    'Build mode:',
    BUILD_MODES.map((m) => ({ name: m, message: MODE_LABELS[m] })),
    base.mode ?? 'local-cached',
  )) as BuildMode;

  let variant: RemoteVariant | undefined;
  if (mode === 'remote') {
    variant = (await select(
      'variant',
      'Image variant:',
      REMOTE_VARIANTS.map((v) => ({ name: v, message: VARIANT_LABELS[v] })),
      base.remote?.variant ?? 'ssh',
    )) as RemoteVariant;
  }

  const modules: SelectedModule[] = [];
  if (mode === 'local-cached') {
    const selectableModules = dockerfileModules.filter((m) => !m.always);
    const selectedIds = await multiselect(
      'modules',
      'Select Dockerfile modules (Space to select, Enter to confirm):',
      selectableModules.map((m) => ({ name: m.id, message: m.label })),
      base.dockerfile.modules.map((m) => m.id),
    );
    for (const id of selectedIds) {
      const mod = getDockerfileModule(id)!;
      const opts: Record<string, unknown> = {};
      for (const o of mod.options ?? []) {
        opts[o.id] = await promptOption(o);
      }
      modules.push({ id, options: opts });
    }
  }

  const selectedModuleIds = new Set(modules.map((m) => m.id));

  const services: SelectedModule[] = [];
  // Always-on services (e.g. the devcontainer itself) are not in the
  // multiselect list, but may still expose options (e.g. dockerSocket mode).
  for (const svc of composeServices.filter((s) => s.always)) {
    if (!svc.options?.length) continue;
    const opts: Record<string, unknown> = {};
    const prev = base.compose.services.find(
      (s) => (typeof s === 'string' ? s : s.id) === svc.id,
    );
    const prevOpts = typeof prev === 'object' && prev ? prev.options ?? {} : {};
    for (const o of svc.options) {
      // Skip options gated on a module that wasn't selected (e.g. Docker
      // access mode only makes sense when the `dod` CLI is installed).
      if (o.requiresModule && !selectedModuleIds.has(o.requiresModule)) continue;
      opts[o.id] = await promptOption({
        ...o,
        default: prevOpts[o.id] ?? o.default,
      });
    }
    services.push({ id: svc.id, options: opts });
  }
  {
    const selectableServices = composeServices.filter(
      (s) =>
        !s.always &&
        !s.internal &&
        (!s.requiresModule || selectedModuleIds.has(s.requiresModule)),
    );
    const baseServiceIds = base.compose.services.map((s) =>
      typeof s === 'string' ? s : s.id,
    );
    const serviceIds = await multiselect(
      'services',
      'Select compose services:',
      selectableServices.map((s) => ({ name: s.id, message: s.label })),
      baseServiceIds,
    );
    for (const id of serviceIds) {
      const svc = composeServices.find((s) => s.id === id)!;
      const opts: Record<string, unknown> = {};
      const prev = base.compose.services.find(
        (s) => (typeof s === 'string' ? s : s.id) === id,
      );
      const prevOpts = typeof prev === 'object' && prev ? prev.options ?? {} : {};
      for (const o of svc.options ?? []) {
        opts[o.id] = await promptOption({
          ...o,
          default: prevOpts[o.id] ?? o.default,
        });
      }
      services.push({ id, options: opts });
    }
  }

  let image: string;
  if (mode === 'remote' && variant) {
    image = resolveRemoteImage(variant, undefined, base.remote?.registry);
  } else {
    image = await input(
      'image',
      'Image name:',
      base.image,
      (v) => (isValidImageName(v) ? true : 'Invalid Docker image name'),
    );
  }

  let subnet: string | undefined;
  {
    const usedSubnets = listUsedSubnets();
    const preferredSubnet = base.compose.subnet ?? '172.25.0.0/28';
    const suggestedSubnet = findFreeSubnet(preferredSubnet, usedSubnets);
    if (suggestedSubnet !== preferredSubnet) {
      console.log(
        chalk.yellow(
          `Subnet ${preferredSubnet} overlaps with existing Docker network. Suggesting ${suggestedSubnet}.`,
        ),
      );
    }
    subnet = await input(
      'subnet',
      'Docker network subnet (CIDR):',
      suggestedSubnet,
      (v) => {
        if (!isValidCidr(v)) return 'Invalid CIDR';
        const clash = subnetConflict(v, usedSubnets);
        if (clash) return `Overlaps with existing network ${formatCidr(clash)}`;
        return true;
      },
    );
  }

  const env: Record<string, string> = { ...base.env };
  const cfgDraft: DevcontainerConfig = {
    mode,
    image,
    workspace,
    dockerfile: { modules },
    compose: { services, subnet },
    env,
  };
  if (mode === 'remote' && variant) {
    cfgDraft.remote = { variant, registry: base.remote?.registry };
  }
  for (const e of collectRequiredEnvVars(cfgDraft)) {
    env[e.name] = await input(e.name, `${e.prompt}:`, env[e.name] ?? e.default ?? '');
  }
  cfgDraft.env = env;
  return cfgDraft;
}

async function promptOption(o: ModuleOption): Promise<unknown> {
  switch (o.type) {
    case 'select':
      return select(
        o.id,
        `  ${o.label}:`,
        (o.choices ?? []).map((c) => ({ name: c.value, message: c.label })),
        o.default as string | undefined,
      );
    case 'multiselect':
      return multiselect(
        o.id,
        `  ${o.label}:`,
        (o.choices ?? []).map((c) => ({ name: c.value, message: c.label })),
        (o.default as string[] | undefined) ?? [],
      );
    case 'confirm':
      return confirm(o.id, `  ${o.label}?`, (o.default as boolean | undefined) ?? true);
    case 'input':
      return input(o.id, `  ${o.label}:`, (o.default as string | undefined) ?? '');
  }
}

function applyFlags(config: DevcontainerConfig, flags: CliFlags): DevcontainerConfig {
  if (flags.mode) config.mode = flags.mode;
  if (flags.workspace) config.workspace = flags.workspace;
  if (flags.withModules) {
    config.dockerfile.modules = flags.withModules.map((id) => {
      const existing = config.dockerfile.modules.find((m) => m.id === id);
      return existing ?? { id, options: {} };
    });
  }
  if (flags.services) {
    config.compose.services = flags.services.map((id) => {
      const existing = config.compose.services.find(
        (s) => (typeof s === 'string' ? s : s.id) === id,
      );
      if (existing && typeof existing === 'object') return existing;
      return { id, options: {} };
    });
  }
  if (flags.mode === 'remote' || config.mode === 'remote') {
    const variant = flags.variant ?? config.remote?.variant ?? 'ssh';
    config.remote = {
      variant,
      registry: flags.registry ?? config.remote?.registry,
    };
  }
  if (flags.image) {
    config.image = flags.image;
  } else if (config.mode === 'remote' && config.remote) {
    config.image = resolveRemoteImage(
      config.remote.variant,
      flags.registry,
      config.remote.registry,
    );
  }
  return config;
}

function writeOutput(filePath: string, content: string, force: boolean): boolean {
  if (fs.existsSync(filePath) && !isGeneratedFile(filePath) && !force) {
    return false;
  }
  fs.writeFileSync(filePath, content);
  return true;
}

async function maybeOverwrite(filePath: string, label: string, interactive: boolean, force: boolean): Promise<boolean> {
  if (force) return true;
  if (!fs.existsSync(filePath)) return true;
  if (isGeneratedFile(filePath)) return true;
  if (!interactive) {
    throw new Error(`${label} exists and is not auto-generated. Use --force or remove it.`);
  }
  return confirm('overwrite', `${label} exists and was not generated by this CLI. Overwrite?`, false);
}

function executeBuild(composePath: string): boolean {
  const status = dockerCompose(composePath, ['build']);
  return status === 0;
}

function executePull(composePath: string): boolean {
  const status = dockerCompose(composePath, ['pull']);
  return status === 0;
}



function printLayoutMessage(workspace: string, hasPostScripts: boolean): void {
  const bar = chalk.gray('─'.repeat(64));
  const root = `.dc_${workspace}`;
  console.log('\n' + bar);
  console.log(chalk.cyan.bold(`Generated layout under ${root}/`));
  console.log(bar);
  console.log(`  ${chalk.bold('build/')}        Dockerfile, docker-compose.yml, .env, helper .sh`);
  console.log(`               ${chalk.gray('→ docker compose -f ' + root + '/build/docker-compose.yml up -d')}`);
  if (hasPostScripts) {
    console.log(`  ${chalk.bold('post-script')}  Baked into the image, run them inside the container`);
    console.log(`               ${chalk.gray('→ ~/post-script/<script>.sh')}`);
  }
  console.log(bar + '\n');
}

export async function runGenerate(argv: string[]): Promise<void> {
  const flags = parseFlags(argv);
  if (flags.help) {
    console.log(helpText());
    return;
  }
  if (flags.version) {
    console.log(getCurrentVersion());
    return;
  }

  console.log(chalk.green.bold('\nDevContainer Dockerfile Builder\n'));

  const cwd = process.cwd();
  const existing = loadConfig(cwd);
  let config = existing ?? defaultConfig();

  const needsPrompts =
    flags.forcePrompt || (!existing && flags.interactive && !flags.withModules && !flags.mode);

  if (needsPrompts) {
    config = await buildConfigFromPrompts(config);
  } else if (
    flags.interactive &&
    flags.mode === 'remote' &&
    !flags.variant &&
    !config.remote?.variant
  ) {
    const picked = (await select(
      'variant',
      'Image variant:',
      REMOTE_VARIANTS.map((v) => ({ name: v, message: VARIANT_LABELS[v] })),
      'ssh',
    )) as RemoteVariant;
    flags.variant = picked;
  }

  config = applyFlags(config, flags);

  if (!config.workspace) {
    config.workspace = resolveWorkspace(cwd, config);
  }
  if (!isValidDockerName(config.workspace)) {
    throw new Error(`Invalid workspace name: ${config.workspace}`);
  }

  if (config.mode === 'remote' && !config.image && config.remote?.variant) {
    config.image = resolveRemoteImage(
      config.remote.variant,
      flags.registry,
      config.remote.registry,
    );
  }

  if (config.mode === 'remote' && !config.remote) {
    throw new Error(`mode=remote requires --variant (one of: ${REMOTE_VARIANTS.join(', ')})`);
  }

  if (!flags.interactive) {
    if (!isValidImageName(config.image)) {
      throw new Error(`Invalid image name: ${config.image}`);
    }
    if (config.compose.subnet && !isValidCidr(config.compose.subnet)) {
      throw new Error(`Invalid CIDR: ${config.compose.subnet}`);
    }
  }

  if (config.compose.subnet) {
    const clash = subnetConflict(config.compose.subnet);
    if (clash) {
      const free = findFreeSubnet(config.compose.subnet);
      if (flags.interactive) {
        console.log(
          chalk.yellow(
            `Subnet ${config.compose.subnet} overlaps with existing Docker network ${formatCidr(clash)}. Using ${free}.`,
          ),
        );
        config.compose.subnet = free;
      } else {
        throw new Error(
          `Subnet ${config.compose.subnet} overlaps with existing Docker network ${formatCidr(clash)}. Set subnet to ${free} in devcontainer.config.json or remove the conflicting network.`,
        );
      }
    }
  }

  const devSvc = config.compose.services.find(
    (s) => typeof s === 'object' && s.id === 'devcontainer',
  );
  const socketMode =
    typeof devSvc === 'object' && (devSvc.options?.dockerSocket as DockerSocketMode) === 'socket';
  if (socketMode && !fs.existsSync(HOST_DOCKER_SOCKET)) {
    const msg = `Docker access mode 'socket' is selected but ${HOST_DOCKER_SOCKET} does not exist on this host. Docker would create an empty directory there on 'up' and docker commands inside the container would fail. Start the Docker daemon, or pick the 'dind'/'none' access mode.`;
    console.log(chalk.yellow(`\nWarning: ${msg}`));
    if (flags.interactive) {
      const ok = await confirm('proceedNoSocket', 'Continue anyway?', false);
      if (!ok) throw new Error('Aborted: host Docker socket not found.');
    }
  }

  const conflicts = findConflicts(config);
  if (conflicts.length > 0) {
    console.log(chalk.yellow('\nDocker name conflicts detected:'));
    for (const c of conflicts) {
      console.log(chalk.yellow(`   - ${c.kind} '${c.name}' already exists (project: ${c.owner})`));
    }
    if (flags.interactive) {
      const ok = await confirm('proceedConflict', 'Continue anyway?', false);
      if (!ok) throw new Error('Aborted due to name conflicts. Change workspace name or remove existing resources.');
    } else {
      throw new Error('Docker name conflicts. Change workspace name or remove existing resources.');
    }
  }

  const paths = projectPaths(cwd, config.workspace);
  const projectDir = paths.projectDir;
  const buildDir = paths.buildDir;

  const skipBuildArtifacts = config.mode === 'remote';

  // Validate every referenced static asset up front — before creating any
  // directories or writing files — so a missing script fails fast instead of
  // half-way through generation.
  if (!skipBuildArtifacts) {
    const referenced = [
      ...collectRequiredCopyFiles(config),
      ...collectRequiredPostScriptFiles(config),
    ];
    const missing = validateRequiredFiles(referenced, buildDir);
    if (missing.length > 0) {
      throw new Error(
        `Missing required script(s) (not embedded in binary, assets/, or ${buildDir}): ${missing.join(', ')}`,
      );
    }
  }

  fs.mkdirSync(buildDir, { recursive: true });

  const copyFiles = skipBuildArtifacts ? [] : collectRequiredCopyFiles(config);
  if (copyFiles.length > 0) {
    const pre = preflight(copyFiles, buildDir);
    if (pre.copied.length > 0) {
      console.log(chalk.gray(`Copied build helpers → .dc_${config.workspace}/build/: ${pre.copied.join(', ')}`));
    }
    if (pre.missing.length > 0) {
      throw new Error(
        `Missing required scripts (not embedded in binary or available in cwd): ${pre.missing.join(', ')}`,
      );
    }
  }

  const postScriptFiles = skipBuildArtifacts ? [] : collectRequiredPostScriptFiles(config);
  if (postScriptFiles.length > 0) {
    const { preflight } = await import('@/infra/preflight.js');
    const postPre = preflight(postScriptFiles, buildDir);
    if (postPre.copied.length > 0) {
      console.log(chalk.gray(`Copied post-install scripts → .dc_${config.workspace}/build/: ${postPre.copied.join(', ')}`));
    }
    if (postPre.missing.length > 0) {
      throw new Error(
        `Missing required post-install scripts: ${postPre.missing.join(', ')}`,
      );
    }
  }

  const dockerfilePath = paths.dockerfilePath;
  const composePath = paths.composeFile;
  const envPath = paths.envPath;

  const dockerfileContent = generateDockerfile(config);

  let cachedImageHit = false;
  if (config.mode === 'local-cached' && dockerfileContent !== null) {
    const copyContents: Record<string, string> = {};
    for (const f of [...copyFiles, ...postScriptFiles]) {
      const onDisk = path.join(buildDir, f);
      if (fs.existsSync(onDisk)) {
        copyContents[f] = fs.readFileSync(onDisk, 'utf8');
      }
    }
    const moduleIds = config.dockerfile.modules.map((m) => m.id);
    const fp = computeFingerprint(dockerfileContent, copyContents, moduleIds);
    config.fingerprint = fp;
    config.image = fingerprintTag(fp);
    if (localImageExists(config.image)) {
      cachedImageHit = true;
      console.log(chalk.gray(`Using cached image ${config.image} (fingerprint ${fp.slice(0, 12)})`));
    }
  }

  if (dockerfileContent === null) {
    console.log(chalk.gray(`Skipped Dockerfile (mode=${config.mode}).`));
  } else if (!(await maybeOverwrite(dockerfilePath, 'Dockerfile', flags.interactive, flags.force))) {
    console.log(chalk.yellow('Skipped Dockerfile.'));
  } else {
    writeOutput(dockerfilePath, dockerfileContent, true);
    console.log(chalk.green('Dockerfile generated.'));
  }

  const composeContent = generateCompose(config);
  if (composeContent === null) {
    console.log(chalk.gray(`Skipped docker-compose.yml (mode=${config.mode}).`));
  } else if (!(await maybeOverwrite(composePath, 'docker-compose.yml', flags.interactive, flags.force))) {
    console.log(chalk.yellow('Skipped docker-compose.yml.'));
  } else {
    writeOutput(composePath, composeContent, true);
    console.log(chalk.green('docker-compose.yml generated.'));
  }

  if (Object.keys(config.env).length > 0 || config.compose.subnet) {
    if (fs.existsSync(envPath) && flags.interactive) {
      const ok = await confirm('overwriteEnv', '.env exists. Overwrite?', false);
      if (ok) fs.writeFileSync(envPath, generateEnv(config));
    } else {
      fs.writeFileSync(envPath, generateEnv(config));
    }
    console.log(chalk.green('.env written.'));
  }

  saveConfig(config, cwd);
  console.log(chalk.green(`Saved devcontainer.config.json`));

  printLayoutMessage(config.workspace, postScriptFiles.length > 0);

  const isRemote = config.mode === 'remote';
  let build = flags.build;
  if (cachedImageHit && build === null) {
    build = false;
    console.log(chalk.green('✓ Local-cached image is up to date — skipping build.'));
  }
  if (build === null && flags.interactive) {
    const label = isRemote
      ? "Run 'docker compose pull' now?"
      : "Run 'docker compose build' now?";
    build = await confirm('build', label, true);
  }

  if (build) {
    const action = isRemote ? executePull : executeBuild;
    const banner = isRemote ? '\nPulling image...\n' : '\nBuilding...\n';
    console.log(chalk.yellow(banner));
    const buildOk = action(composePath);
    if (buildOk) {
      recordProject(cwd, config);
      printSshInstructions(config.workspace, config.mode);
    }
  } else {
    recordProject(cwd, config);
    console.log(chalk.green.bold('\nDone.\n'));
    printSshInstructions(config.workspace, config.mode);
  }
}
