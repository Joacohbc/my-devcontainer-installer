import * as fs from 'fs';
import * as path from 'path';
import { exec } from 'child_process';
import chalk from 'chalk';

import { parseFlags, helpText, type CliFlags } from './cli.js';
import { defaultConfig, loadConfig, saveConfig } from './config.js';
import {
  generateDockerfile,
  generateCompose,
  generateEnv,
  collectRequiredCopyFiles,
  collectRequiredEnvVars,
  collectRequiredPostScriptFiles,
  resolveDevcontainerImageName,
  resolveRemoteImage,
} from './generator.js';
import { printCleanupInstructions } from './cleanup-instructions.js';
import { findConflicts } from './docker-conflicts.js';
import { isGeneratedFile, preflight } from './preflight.js';
import { findFreeSubnet, formatCidr, listUsedSubnets, subnetConflict } from './subnet.js';
import { printSshInstructions } from './ssh-instructions.js';
import { confirm, input, multiselect, PromptCancelledError, select } from './prompts.js';
import { composeServices, dockerfileModules, getDockerfileModule } from './registry.js';
import { runSetupSsh } from './setup-ssh.js';
import { cleanupStaleUpdate, getCurrentVersion, runSelfUpdate } from './self-update.js';
import { runUpdateImages } from './update-images.js';
import { runConfigCmd } from './config-cmd.js';
import { runQuickRun } from './quick-run.js';
import { runDown } from './down.js';
import { runDestroy } from './destroy.js';
import { runLifecycle } from './lifecycle.js';
import { runPrune } from './prune.js';
import {
  computeFingerprint,
  fingerprintTag,
  localImageExists,
  recordEntry,
} from './image-registry.js';
import {
  BUILD_MODES,
  REMOTE_VARIANTS,
  type BuildMode,
  type DevcontainerConfig,
  type ModuleOption,
  type RemoteVariant,
  type SelectedModule,
} from './types.js';
import { isValidCidr, isValidDockerName, isValidImageName, sanitizeDockerName } from './validators.js';

const MODE_LABELS: Record<BuildMode, string> = {
  'local-cached': 'local-cached — generate Dockerfile + compose, reuse cached image when unchanged',
  remote: 'remote — skip the build, pull a pre-built image from the registry',
};

const VARIANT_LABELS: Record<RemoteVariant, string> = {
  ssh: 'ssh — full image (all modules)',
  nodejs: 'nodejs — Node.js only',
  bun: 'bun — Bun only',
  'java-temurin': 'java-temurin — Java Temurin only',
  python: 'python — Python only',
  go: 'go — Go only',
  'node-go': 'node-go — Node.js + Go',
  'node-python': 'node-python — Node.js + Python',
  'node-java-temurin': 'node-java-temurin — Node.js + Java Temurin',
  'bun-go': 'bun-go — Bun + Go',
  'bun-python': 'bun-python — Bun + Python',
  'bun-java-temurin': 'bun-java-temurin — Bun + Java Temurin',
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

  const services: SelectedModule[] = [];
  {
    const selectableServices = composeServices.filter((s) => !s.always);
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
          `⚠  Subnet ${preferredSubnet} overlaps with existing Docker network. Suggesting ${suggestedSubnet}.`,
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

function executeDockerCommand(cwd: string, command: string, label: string): Promise<boolean> {
  return new Promise((resolve) => {
    const child = exec(command, { cwd });
    child.stdout?.pipe(process.stdout);
    child.stderr?.pipe(process.stderr);
    child.on('close', (code) => {
      if (code !== 0) {
        console.error(chalk.red(`\n❌ ${label} failed (exit ${code})\n`));
        resolve(false);
      } else {
        console.log(chalk.green.bold(`\n✅ ${label} completed!\n`));
        resolve(true);
      }
    });
  });
}

function executeBuild(cwd: string): Promise<boolean> {
  return executeDockerCommand(cwd, 'docker compose build', 'Build');
}

function executePull(cwd: string): Promise<boolean> {
  return executeDockerCommand(cwd, 'docker compose pull', 'Pull');
}

function installSignalHandlers(): void {
  const cancel = () => {
    if (process.stdin.isTTY && process.stdin.setRawMode) {
      try { process.stdin.setRawMode(false); } catch { /* noop */ }
    }
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  };
  process.once('SIGINT', cancel);
  process.once('SIGTERM', cancel);
  process.on('unhandledRejection', (e: unknown) => {
    const err = e as { code?: string; message?: string } | undefined;
    if (!err) return;
    if (err.code === 'ERR_USE_AFTER_CLOSE') return;
    if (err.message === '' || err.message === 'canceled') return;
    console.error(chalk.red(`\n❌ ${err.message ?? String(e)}\n`));
    process.exit(1);
  });
}

async function main() {
  installSignalHandlers();
  cleanupStaleUpdate();
  const argv = process.argv.slice(2);
  if (argv[0] === 'setup-ssh') {
    await runSetupSsh(argv.slice(1));
    return;
  }
  if (argv[0] === 'cleanup-tips') {
    const config = loadConfig(process.cwd()) ?? defaultConfig();
    if (!config.workspace) config.workspace = sanitizeDockerName(path.basename(process.cwd()));
    printCleanupInstructions(config);
    return;
  }
  if (argv[0] === 'update') {
    await runUpdateImages(argv.slice(1));
    return;
  }
  if (argv[0] === 'upgrade-cli') {
    await runSelfUpdate(argv.slice(1));
    return;
  }
  if (argv[0] === 'config') {
    await runConfigCmd(argv.slice(1));
    return;
  }
  if (argv[0] === 'run') {
    await runQuickRun(argv.slice(1));
    return;
  }
  if (argv[0] === 'down') {
    await runDown(argv.slice(1));
    return;
  }
  if (argv[0] === 'destroy') {
    await runDestroy(argv.slice(1));
    return;
  }
  if (argv[0] === 'start' || argv[0] === 'stop' || argv[0] === 'restart') {
    await runLifecycle(argv[0], argv.slice(1));
    return;
  }
  if (argv[0] === 'prune') {
    await runPrune(argv.slice(1));
    return;
  }

  const flags = parseFlags(argv);
  if (flags.help) {
    console.log(helpText());
    return;
  }
  if (flags.version) {
    console.log(getCurrentVersion());
    return;
  }

  console.log(chalk.green.bold('\n🚀 DevContainer Dockerfile Builder\n'));

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
    config.workspace = sanitizeDockerName(path.basename(cwd));
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
            `⚠  Subnet ${config.compose.subnet} overlaps with existing Docker network ${formatCidr(clash)}. Using ${free}.`,
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

  const conflicts = findConflicts(config);
  if (conflicts.length > 0) {
    console.log(chalk.yellow('\n⚠  Docker name conflicts detected:'));
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

  const projectDir = path.join(cwd, `.dc_${config.workspace}`);
  const buildDir = path.join(projectDir, 'build');
  fs.mkdirSync(buildDir, { recursive: true });

  const skipBuildArtifacts = config.mode === 'remote';
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

  // Post-install scripts go into the build context so the generated Dockerfile can
  // COPY them into the image (POST_SCRIPT_DIR). They are baked, not run.
  const postScriptFiles = skipBuildArtifacts ? [] : collectRequiredPostScriptFiles(config);
  if (postScriptFiles.length > 0) {
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

  const dockerfilePath = path.join(buildDir, 'Dockerfile');
  const composePath = path.join(buildDir, 'docker-compose.yml');
  const envPath = path.join(buildDir, '.env');

  const dockerfileContent = generateDockerfile(config);

  // Fingerprint computation (local-cached only): hash Dockerfile + copyFile contents
  // + selected module IDs. Determines the canonical image name and whether we can
  // skip the rebuild.
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
    console.log(chalk.green('✅ Dockerfile generated.'));
  }

  const composeContent = generateCompose(config);
  if (composeContent === null) {
    console.log(chalk.gray(`Skipped docker-compose.yml (mode=${config.mode}).`));
  } else if (!(await maybeOverwrite(composePath, 'docker-compose.yml', flags.interactive, flags.force))) {
    console.log(chalk.yellow('Skipped docker-compose.yml.'));
  } else {
    writeOutput(composePath, composeContent, true);
    console.log(chalk.green('✅ docker-compose.yml generated.'));
  }

  if (Object.keys(config.env).length > 0 || config.compose.subnet) {
    if (fs.existsSync(envPath) && flags.interactive) {
      const ok = await confirm('overwriteEnv', '.env exists. Overwrite?', false);
      if (ok) fs.writeFileSync(envPath, generateEnv(config));
    } else {
      fs.writeFileSync(envPath, generateEnv(config));
    }
    console.log(chalk.green('✅ .env written.'));
  }

  saveConfig(config, cwd);
  console.log(chalk.green(`✅ Saved devcontainer.config.json`));

  printLayoutMessage(config.workspace, postScriptFiles.length > 0);

  const isRemote = config.mode === 'remote';
  let build = flags.build;
  if (cachedImageHit && build === null) {
    // Fast path: image is already in the daemon for this fingerprint.
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
    const banner = isRemote ? '\n⏳ Pulling image...\n' : '\n⏳ Building...\n';
    console.log(chalk.yellow(banner));
    const buildOk = await action(buildDir);
    if (buildOk) {
      recordProjectEntry(cwd, config);
      printSshInstructions(config.workspace, config.mode);
    }
  } else {
    recordProjectEntry(cwd, config);
    console.log(chalk.green.bold('\n✨ Done.\n'));
    printSshInstructions(config.workspace, config.mode);
  }
}

function recordProjectEntry(projectDir: string, config: DevcontainerConfig): void {
  try {
    const now = new Date().toISOString();
    recordEntry({
      projectDir,
      workspace: config.workspace,
      mode: config.mode,
      image: config.image,
      variant: config.remote?.variant,
      fingerprint: config.fingerprint,
      createdAt: now,
      lastUpdated: now,
    });
  } catch {
    // Non-fatal: registry is a convenience for `update --all`.
  }
}

function printLayoutMessage(workspace: string, hasPostScripts: boolean): void {
  const bar = chalk.gray('─'.repeat(64));
  const root = `.dc_${workspace}`;
  console.log('\n' + bar);
  console.log(chalk.cyan.bold(`📁 Generated layout under ${root}/`));
  console.log(bar);
  console.log(`  ${chalk.bold('build/')}        Dockerfile, docker-compose.yml, .env, helper .sh`);
  console.log(`               ${chalk.gray('→ docker compose -f ' + root + '/build/docker-compose.yml up -d')}`);
  if (hasPostScripts) {
    console.log(`  ${chalk.bold('post-script')}  Baked into the image, run them inside the container`);
    console.log(`               ${chalk.gray('→ ~/post-script/<script>.sh')}`);
  }
  console.log(bar + '\n');
}

main().catch((e) => {
  if (e instanceof PromptCancelledError) {
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  }
  console.error(chalk.red(`\n❌ ${e.message ?? e}\n`));
  process.exit(1);
});
