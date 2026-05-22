import * as fs from 'fs';
import * as path from 'path';
import { spawnSync, exec } from 'child_process';
import chalk from 'chalk';
import { confirm, input, multiselect, select } from './prompts.js';
import { dockerfileModules, getDockerfileModule } from './registry.js';
import { loadConfig, saveConfig, defaultConfig } from './config.js';
import { generateDockerfile, collectRequiredCopyFiles, collectRequiredPostScriptFiles } from './generator.js';
import { isGeneratedFile, preflight } from './preflight.js';
import { findFreeSubnet, formatCidr, listUsedSubnets, subnetConflict, lastHost } from './subnet.js';
import { runSetupSsh } from './setup-ssh.js';
import { findConflicts } from './docker-conflicts.js';
import type { DevcontainerConfig, SelectedModule, ModuleOption } from './types.js';
import { isValidCidr, isValidDockerName, isValidImageName, sanitizeDockerName } from './validators.js';

export interface StandaloneCliFlags {
  interactive: boolean;
  forcePrompt: boolean;
  build: boolean | null;
  image?: string;
  workspace?: string;
  withModules?: string[];
  mount?: string;
  subnet?: string;
  force: boolean;
  help: boolean;
}

export function parseStandaloneFlags(argv: string[]): StandaloneCliFlags {
  const flags: StandaloneCliFlags = {
    interactive: true,
    forcePrompt: false,
    force: false,
    build: null,
    help: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => argv[++i];
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '--no-interactive':
      case '--non-interactive':
        flags.interactive = false;
        break;
      case '--force-prompt':
        flags.forcePrompt = true;
        break;
      case '--build':
        flags.build = true;
        break;
      case '--no-build':
        flags.build = false;
        break;
      case '--image':
        flags.image = next();
        break;
      case '--workspace':
        flags.workspace = next();
        break;
      case '--with':
        flags.withModules = next().split(',').map((s) => s.trim()).filter(Boolean);
        break;
      case '--mount':
        flags.mount = next();
        break;
      case '--subnet':
        flags.subnet = next();
        break;
      case '--force':
        flags.force = true;
        break;
      default:
        if (a.startsWith('--')) {
          throw new Error(`Unknown flag: ${a}`);
        }
    }
  }
  return flags;
}

export function standaloneHelpText(): string {
  return `devcontainer CLI — generate Dockerfile for standalone container (no Compose)

Usage:
  cli standalone [flags]

Flags:
  --with <ids>          Comma-separated dockerfile modules (e.g. nodejs,java,dod)
  --image <name>        Image name (default: <workspace>:local)
  --workspace <name>    Workspace name (default: current dir name, used for network/container names)
  --mount <path>        Host path to mount inside the container at /workspace
  --subnet <cidr>       Docker network subnet (CIDR) (default: 172.25.0.0/28)
  --no-interactive      Fail if any value is missing instead of prompting
  --force-prompt        Prompt even if config file exists
  --force               Overwrite existing files without prompting
  --build / --no-build  Run 'docker build' after generating (default: ask)
  -h, --help            Show this help
`;
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

async function buildStandaloneConfigFromPrompts(base: DevcontainerConfig): Promise<DevcontainerConfig> {
  const workspace = await input(
    'workspace',
    'Workspace name (used as prefix for network and container):',
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

  let mount = base.standalone?.mount || '';
  const hasMount = await confirm('hasMount', 'Do you want to mount a host directory inside the container?', mount !== '');
  if (hasMount) {
    mount = await input(
      'mount',
      'Host directory path to mount (will be mounted at /workspace):',
      mount || process.cwd()
    );
  } else {
    mount = '';
  }

  const image = await input(
    'image',
    'Image name:',
    base.image,
    (v) => (isValidImageName(v) ? true : 'Invalid Docker image name'),
  );

  const usedSubnets = listUsedSubnets();
  const preferredSubnet = base.standalone?.subnet || base.compose?.subnet || '172.25.0.0/28';
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

  return {
    mode: 'standalone',
    image,
    workspace,
    dockerfile: { modules },
    compose: base.compose,
    standalone: {
      subnet,
      mount: mount || undefined,
    },
    env: base.env,
  };
}

function applyStandaloneFlags(config: DevcontainerConfig, flags: StandaloneCliFlags): DevcontainerConfig {
  if (flags.image) config.image = flags.image;
  if (flags.workspace) config.workspace = flags.workspace;
  if (flags.withModules) {
    config.dockerfile.modules = flags.withModules.map((id) => {
      const existing = config.dockerfile.modules.find((m) => m.id === id);
      return existing ?? { id, options: {} };
    });
  }
  if (!config.standalone) {
    config.standalone = {};
  }
  if (flags.mount !== undefined) {
    config.standalone.mount = flags.mount || undefined;
  }
  if (flags.subnet) {
    config.standalone.subnet = flags.subnet;
  }
  config.mode = 'standalone';
  return config;
}

function executeStandaloneBuild(cwd: string, imageName: string): Promise<boolean> {
  return new Promise((resolve) => {
    const child = exec(`docker build -t ${imageName} .`, { cwd });
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

function createDockerNetwork(networkName: string, subnet: string): void {
  const inspect = spawnSync('docker', ['network', 'inspect', networkName], { encoding: 'utf8' });
  if (inspect.status === 0) {
    console.log(chalk.gray(`Docker network '${networkName}' already exists.`));
    return;
  }
  console.log(chalk.yellow(`Creating Docker network '${networkName}' with subnet ${subnet}...`));
  const create = spawnSync('docker', ['network', 'create', '--subnet', subnet, networkName], { encoding: 'utf8' });
  if (create.status !== 0) {
    console.error(chalk.red(`❌ Failed to create Docker network '${networkName}': ${create.stderr}`));
  } else {
    console.log(chalk.green(`✅ Docker network '${networkName}' created.`));
  }
}

export function generateDockerRunCommand(config: DevcontainerConfig): { runCommand: string; networkCommand: string } {
  const containerName = `${config.workspace}-devcontainer-ssh`;
  const networkName = `${config.workspace}-network`;
  const subnet = config.standalone?.subnet || '172.25.0.0/28';
  const ip = lastHost(subnet) || '172.25.0.14';

  const networkCommand = `docker network create --subnet ${subnet} ${networkName}`;

  const args = [
    'docker run -d',
    `  --name ${containerName}`,
    `  --network ${networkName}`,
    `  --ip ${ip}`,
    '  -v /var/run/docker.sock:/var/run/docker.sock',
  ];

  if (config.standalone?.mount) {
    const hostPath = path.resolve(config.standalone.mount);
    args.push(`  -v ${hostPath}:/workspace`);
  }

  args.push(`  ${config.image}`);
  args.push('  sleep infinity');

  return {
    networkCommand,
    runCommand: args.join(' \\\n'),
  };
}

export async function runStandalone(argv: string[]): Promise<void> {
  const flags = parseStandaloneFlags(argv);
  if (flags.help) {
    console.log(standaloneHelpText());
    return;
  }

  console.log(chalk.green.bold('\n🚀 DevContainer Standalone Docker Builder (No Compose)\n'));

  const cwd = process.cwd();
  const existing = loadConfig(cwd);
  let config = existing ?? defaultConfig();

  const needsPrompts =
    flags.forcePrompt || (!existing && flags.interactive && !flags.withModules);

  if (needsPrompts) {
    config = await buildStandaloneConfigFromPrompts(config);
  } else {
    config = applyStandaloneFlags(config, flags);
  }

  if (!config.workspace) {
    config.workspace = sanitizeDockerName(path.basename(cwd));
  }
  if (!isValidDockerName(config.workspace)) {
    throw new Error(`Invalid workspace name: ${config.workspace}`);
  }

  const containerName = `${config.workspace}-devcontainer-ssh`;
  const networkName = `${config.workspace}-network`;
  const subnet = config.standalone?.subnet || '172.25.0.0/28';

  if (!flags.interactive) {
    if (!isValidImageName(config.image)) {
      throw new Error(`Invalid image name: ${config.image}`);
    }
    if (subnet && !isValidCidr(subnet)) {
      throw new Error(`Invalid CIDR: ${subnet}`);
    }
  }

  if (subnet) {
    const clash = subnetConflict(subnet);
    if (clash) {
      const free = findFreeSubnet(subnet);
      if (flags.interactive) {
        console.log(
          chalk.yellow(
            `⚠  Subnet ${subnet} overlaps with existing Docker network ${formatCidr(clash)}. Using ${free}.`,
          ),
        );
        if (!config.standalone) config.standalone = {};
        config.standalone.subnet = free;
      } else {
        throw new Error(
          `Subnet ${subnet} overlaps with existing Docker network ${formatCidr(clash)}. Set subnet to ${free} in devcontainer.config.json or remove the conflicting network.`,
        );
      }
    }
  }

  // Standalone mode conflict checks
  const conflicts = findConflicts(config);
  const standaloneConflicts = conflicts.filter(
    (c) => c.name === containerName || c.name === networkName
  );
  if (standaloneConflicts.length > 0) {
    console.log(chalk.yellow('\n⚠  Docker name conflicts detected:'));
    for (const c of standaloneConflicts) {
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
  const postScriptDir = path.join(projectDir, 'post-script');
  fs.mkdirSync(buildDir, { recursive: true });

  const copyFiles = collectRequiredCopyFiles(config);
  const pre = preflight(copyFiles, buildDir);
  if (pre.copied.length > 0) {
    console.log(chalk.gray(`Copied build helpers → .dc_${config.workspace}/build/: ${pre.copied.join(', ')}`));
  }
  if (pre.missing.length > 0) {
    throw new Error(
      `Missing required scripts (not embedded in binary or available in cwd): ${pre.missing.join(', ')}`,
    );
  }

  const postScriptFiles = collectRequiredPostScriptFiles(config);
  if (postScriptFiles.length > 0) {
    fs.mkdirSync(postScriptDir, { recursive: true });
    const postPre = preflight(postScriptFiles, postScriptDir);
    if (postPre.copied.length > 0) {
      console.log(chalk.gray(`Copied post-install scripts → .dc_${config.workspace}/post-script/: ${postPre.copied.join(', ')}`));
    }
    if (postPre.missing.length > 0) {
      throw new Error(
        `Missing required post-install scripts: ${postPre.missing.join(', ')}`,
      );
    }
  }

  const dockerfilePath = path.join(buildDir, 'Dockerfile');
  const dockerfileExists = fs.existsSync(dockerfilePath);
  const canWriteDockerfile = !dockerfileExists || isGeneratedFile(dockerfilePath) || flags.force;

  let shouldWrite = true;
  if (dockerfileExists && !canWriteDockerfile) {
    if (flags.interactive) {
      shouldWrite = await confirm('overwriteDockerfile', 'Dockerfile exists and was not generated by this CLI. Overwrite?', false);
    } else {
      throw new Error(`Dockerfile exists and is not auto-generated. Use --force or remove it.`);
    }
  }

  if (shouldWrite) {
    fs.writeFileSync(dockerfilePath, generateDockerfile(config));
    console.log(chalk.green('✅ Dockerfile generated.'));
  } else {
    console.log(chalk.yellow('Skipped Dockerfile.'));
  }

  saveConfig(config, cwd);
  console.log(chalk.green(`✅ Saved devcontainer.config.json`));

  let build = flags.build;
  if (build === null && flags.interactive) {
    build = await confirm('build', "Run 'docker build' now?", true);
  }

  const { networkCommand, runCommand } = generateDockerRunCommand(config);

  if (build) {
    console.log(chalk.yellow('\n⏳ Building image...\n'));
    const ok = await executeStandaloneBuild(buildDir, config.image);
    if (ok) {
      // Automatical network creation on build
      createDockerNetwork(networkName, config.standalone?.subnet || '172.25.0.0/28');

      console.log('\n' + chalk.cyan.bold('🚀 Docker Network & Container Run Command:'));
      console.log(chalk.gray('─'.repeat(64)));
      console.log(chalk.bold('Run the container with:'));
      console.log(chalk.cyan(runCommand));
      console.log(chalk.gray('─'.repeat(64)) + '\n');

      if (flags.interactive) {
        const runNow = await confirm('runNow', 'Do you want to run the container now?', true);
        if (runNow) {
          const runCmdPlain = runCommand.replace(/\\\n/g, ' ').replace(/\s+/g, ' ').trim();
          console.log(chalk.yellow('Starting container...'));
          const runRes = spawnSync('sh', ['-c', runCmdPlain], { stdio: 'inherit' });
          if (runRes.status === 0) {
            console.log(chalk.green(`✅ Container '${containerName}' started.`));
            const setupNow = await confirm('setupSshNow', 'Do you want to run setup-ssh now?', true);
            if (setupNow) {
              console.log(chalk.yellow('\n⏳ Running setup-ssh...\n'));
              await runSetupSsh([
                '--container', containerName,
                '--alias', config.workspace,
                '--compose-file', 'does-not-exist', // bypass compose lookup
                '--user', 'devuser',
                '-y'
              ]);
            }
          } else {
            console.error(chalk.red('❌ Failed to run container.'));
          }
        }
      }
    }
  } else {
    console.log('\n' + chalk.cyan.bold('🚀 Docker Network & Container Run Commands:'));
    console.log(chalk.gray('─'.repeat(64)));
    console.log(chalk.bold('1) Create the network:'));
    console.log(chalk.cyan(`   $ ${networkCommand}`));
    console.log('\n' + chalk.bold('2) Run the container:'));
    console.log(chalk.cyan(runCommand));
    console.log(chalk.gray('─'.repeat(64)) + '\n');
  }
}
