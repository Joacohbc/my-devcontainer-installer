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
} from './generator.js';
import { printCleanupInstructions } from './cleanup-instructions.js';
import { findConflicts } from './docker-conflicts.js';
import { isGeneratedFile, preflight } from './preflight.js';
import { findFreeSubnet, formatCidr, listUsedSubnets, subnetConflict } from './subnet.js';
import { printSshInstructions } from './ssh-instructions.js';
import { confirm, input, multiselect, PromptCancelledError, select } from './prompts.js';
import { composeServices, dockerfileModules, getDockerfileModule } from './registry.js';
import { runSetupSsh } from './setup-ssh.js';
import type { DevcontainerConfig, ModuleOption, SelectedModule } from './types.js';
import { isValidCidr, isValidDockerName, isValidImageName, sanitizeDockerName } from './validators.js';

async function buildConfigFromPrompts(base: DevcontainerConfig): Promise<DevcontainerConfig> {
  const workspace = await input(
    'workspace',
    'Workspace name (used as prefix for containers, network, volumes):',
    base.workspace || sanitizeDockerName(path.basename(process.cwd())),
    (v) => (isValidDockerName(v) ? true : 'Invalid Docker name (allowed: a-z A-Z 0-9 _ . -, ≤63 chars, start alphanumeric)'),
  );
  base.workspace = workspace;

  const selectableModules = dockerfileModules.filter((m) => !m.always);
  const selectedIds = await multiselect(
    'modules',
    'Select Dockerfile modules (Space to select, Enter to confirm):',
    selectableModules.map((m) => ({ name: m.id, message: m.label })),
    base.dockerfile.modules.map((m) => m.id),
  );

  const modules: SelectedModule[] = [];
  for (const id of selectedIds) {
    const mod = getDockerfileModule(id)!;
    const opts: Record<string, unknown> = {};
    for (const o of mod.options ?? []) {
      opts[o.id] = await promptOption(o);
    }
    modules.push({ id, options: opts });
  }

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

  const services: SelectedModule[] = [];
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

  const image = await input(
    'image',
    'Image name:',
    base.image,
    (v) => (isValidImageName(v) ? true : 'Invalid Docker image name'),
  );

  const usedSubnets = listUsedSubnets();
  const preferredSubnet = base.compose.subnet ?? '172.25.0.0/24';
  const suggestedSubnet = findFreeSubnet(preferredSubnet, usedSubnets);
  if (suggestedSubnet !== preferredSubnet) {
    console.log(
      chalk.yellow(
        `⚠  Subnet ${preferredSubnet} overlaps with existing Docker network. Suggesting ${suggestedSubnet}.`,
      ),
    );
  }
  const subnet = await input(
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

  const env: Record<string, string> = { ...base.env };
  const cfgDraft: DevcontainerConfig = {
    image,
    workspace,
    dockerfile: { modules },
    compose: { services, subnet },
    env,
  };
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
  if (flags.image) config.image = flags.image;
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
  return config;
}

function writeOutput(filePath: string, content: string, force: boolean): boolean {
  if (fs.existsSync(filePath) && !isGeneratedFile(filePath) && !force) {
    return false;
  }
  fs.writeFileSync(filePath, content);
  return true;
}

async function maybeOverwrite(filePath: string, label: string, interactive: boolean): Promise<boolean> {
  if (!fs.existsSync(filePath)) return true;
  if (isGeneratedFile(filePath)) return true;
  if (!interactive) {
    throw new Error(`${label} exists and is not auto-generated. Use --force-prompt or remove it.`);
  }
  return confirm('overwrite', `${label} exists and was not generated by this CLI. Overwrite?`, false);
}

function executeBuild(cwd: string): Promise<boolean> {
  return new Promise((resolve) => {
    const child = exec('docker compose build', { cwd });
    child.stdout?.pipe(process.stdout);
    child.stderr?.pipe(process.stderr);
    child.on('close', (code) => {
      if (code !== 0) {
        console.error(chalk.red(`\n❌ Build failed (exit ${code})\n`));
        resolve(false);
      } else {
        console.log(chalk.green.bold('\n✅ Build completed!\n'));
        resolve(true);
      }
    });
  });
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
  const argv = process.argv.slice(2);
  if (argv[0] === 'setup-ssh') {
    await runSetupSsh(argv.slice(1));
    return;
  }

  const flags = parseFlags(argv);
  if (flags.help) {
    console.log(helpText());
    return;
  }

  console.log(chalk.green.bold('\n🚀 DevContainer Dockerfile Builder\n'));

  const cwd = process.cwd();
  const existing = loadConfig(cwd);
  let config = existing ?? defaultConfig();

  const needsPrompts =
    flags.forcePrompt || (!existing && flags.interactive && !flags.withModules);

  if (needsPrompts) {
    config = await buildConfigFromPrompts(config);
  }

  config = applyFlags(config, flags);

  if (!config.workspace) {
    config.workspace = sanitizeDockerName(path.basename(cwd));
  }
  if (!isValidDockerName(config.workspace)) {
    throw new Error(`Invalid workspace name: ${config.workspace}`);
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

  const copyFiles = collectRequiredCopyFiles(config);
  const pre = preflight(copyFiles, cwd);
  if (pre.copied.length > 0) {
    console.log(chalk.gray(`Copied helper files: ${pre.copied.join(', ')}`));
  }
  if (pre.missing.length > 0) {
    throw new Error(
      `Missing required files (not in cwd or cli/assets): ${pre.missing.join(', ')}`,
    );
  }

  const dockerfilePath = path.join(cwd, 'Dockerfile');
  const composePath = path.join(cwd, 'docker-compose.yml');
  const envPath = path.join(cwd, '.env');

  if (!(await maybeOverwrite(dockerfilePath, 'Dockerfile', flags.interactive))) {
    console.log(chalk.yellow('Skipped Dockerfile.'));
  } else {
    writeOutput(dockerfilePath, generateDockerfile(config), true);
    console.log(chalk.green('✅ Dockerfile generated.'));
  }

  if (!(await maybeOverwrite(composePath, 'docker-compose.yml', flags.interactive))) {
    console.log(chalk.yellow('Skipped docker-compose.yml.'));
  } else {
    writeOutput(composePath, generateCompose(config), true);
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

  let build = flags.build;
  if (build === null && flags.interactive) {
    build = await confirm('build', "Run 'docker compose build' now?", true);
  }
  if (build) {
    console.log(chalk.yellow('\n⏳ Building...\n'));
    const ok = await executeBuild(cwd);
    if (ok) {
      printSshInstructions();
      printCleanupInstructions(config);
    }
  } else {
    console.log(chalk.green.bold('\n✨ Done.\n'));
    printSshInstructions();
    printCleanupInstructions(config);
  }
}

main().catch((e) => {
  if (e instanceof PromptCancelledError) {
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  }
  console.error(chalk.red(`\n❌ ${e.message ?? e}\n`));
  process.exit(1);
});
