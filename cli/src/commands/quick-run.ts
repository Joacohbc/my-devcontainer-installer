import chalk from 'chalk';
import { dockerCapture, dockerInherit } from '@/docker.js';
import { select, confirm, PromptCancelledError } from '@/prompts.js';
import { resolveRemoteImage } from '@/generator.js';
import { LABEL_MANAGED, LABEL_QUICK_RUN } from '@/labels.js';
import { REMOTE_VARIANTS, VARIANT_LABELS, parseVariant, type RemoteVariant } from '@/types.js';
import type { Command } from '@/commands/command.js';
export interface QuickRunFlags {
  variant?: RemoteVariant;
  volume?: string;
  name?: string;
  port?: number;
  registry?: string;
  interactive: boolean;
  help: boolean;
}

export function parseQuickRunFlags(argv: string[]): QuickRunFlags {
  const flags: QuickRunFlags = { interactive: true, help: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => argv[++i];
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '--variant':
        flags.variant = parseVariant(next());
        break;
      case '--volume':
        flags.volume = next();
        break;
      case '--name':
        flags.name = next();
        break;
      case '--port': {
        const n = Number(next());
        if (!Number.isInteger(n) || n < 0 || n > 65535) throw new Error('Invalid --port value');
        flags.port = n;
        break;
      }
      case '--registry':
        flags.registry = next();
        break;
      case '--no-interactive':
        flags.interactive = false;
        break;
      default:
        throw new Error(`Unknown flag for run: ${a}`);
    }
  }
  return flags;
}

export function quickRunHelp(): string {
  return `devcontainer-cli run — spin up a container from a remote image without any project files

Usage:
  devcontainer-cli run [flags]

Flags:
  --variant <name>   Image variant: ${REMOTE_VARIANTS.join(', ')}
  --name <name>      Container name (default: dc-<variant>)
  --volume <name>    Named volume to mount at /workspace (optional)
  --port <n>         Expose container port 22 on host port n (optional)
  --registry <url>   Registry prefix override
  --no-interactive   Fail if --variant is missing instead of prompting
  -h, --help         Show this help
`;
}



export async function runQuickRun(argv: string[]): Promise<void> {
  const flags = parseQuickRunFlags(argv);
  if (flags.help) { console.log(quickRunHelp()); return; }

  let variant = flags.variant;
  if (!variant) {
    if (!flags.interactive) throw new Error('--variant required in non-interactive mode');
    variant = (await select(
      'variant',
      'Image variant:',
      REMOTE_VARIANTS.map((v) => ({ name: v, message: VARIANT_LABELS[v] })),
      'ssh',
    )) as RemoteVariant;
  }

  const image = resolveRemoteImage(variant, flags.registry);
  const containerName = flags.name ?? `dc-${variant}`;
  const volumeName = flags.volume;

  console.log(chalk.cyan.bold(`\nQuick run: ${image}\n`));
  console.log(chalk.gray(`  Container : ${containerName}`));
  if (volumeName) console.log(chalk.gray(`  Volume    : ${volumeName} → /workspace`));
  if (flags.port)  console.log(chalk.gray(`  Port      : ${flags.port}:22`));
  console.log('');

  // Check if container already running
  const inspect = dockerCapture(['inspect', '-f', '{{.State.Status}}', containerName]);
  const state = inspect.stdout.replace(/\s/g, '');
  if (state === 'running') {
    console.log(chalk.green(`✓ Container '${containerName}' is already running.`));
    printNextSteps(containerName);
    return;
  }
  if (state === 'exited' || state === 'created' || state === 'paused') {
    console.log(chalk.yellow(`↻ Starting existing container '${containerName}'...`));
    const code = dockerInherit(['start', containerName]);
    if (code !== 0) throw new Error('docker start failed.');
    printNextSteps(containerName);
    return;
  }

  // Build docker run args
  const args: string[] = [
    'run', '-d',
    '--name', containerName,
    '--restart', 'unless-stopped',
    '-v', '/var/run/docker.sock:/var/run/docker.sock',
    '--label', `${LABEL_MANAGED}=true`,
    '--label', `${LABEL_QUICK_RUN}=${variant}`,
  ];
  if (volumeName) {
    // Create volume if it doesn't exist
    dockerInherit(['volume', 'create', volumeName]);
    args.push('-v', `${volumeName}:/workspace`);
  }
  if (flags.port) args.push('-p', `${flags.port}:22`);
  args.push(image, 'sleep', 'infinity');

  console.log(chalk.yellow('Pulling and starting container...\n'));
  const code = dockerInherit(args);
  if (code !== 0) throw new Error('docker run failed.');

  console.log('');
  printNextSteps(containerName);
}

function printNextSteps(containerName: string): void {
  const bar = chalk.gray('─'.repeat(64));
  console.log(bar);
  console.log(chalk.green.bold('Container running.'));
  console.log('');
  console.log(chalk.bold('Next: set up SSH access'));
  console.log(chalk.gray(`   $ devcontainer-cli setup-ssh --container ${containerName}`));
  console.log('');
  console.log(chalk.bold('Post-install scripts (baked into the image, run on demand):'));
  console.log(chalk.gray(`   $ docker exec -it ${containerName} ls ~/post-script`));
  console.log(chalk.gray(`   $ docker exec -it -u devuser ${containerName} bash ~/post-script/login-github-cli.sh`));
  console.log(bar + '\n');
}

export const quickRunCommand: Command<QuickRunFlags> = {
  name: 'run',
  summary: 'Spin up a remote image container without project files',
  parse: parseQuickRunFlags,
  help: quickRunHelp,
  run: runQuickRun,
};
