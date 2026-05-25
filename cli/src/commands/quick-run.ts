import chalk from 'chalk';
import { dockerCapture, dockerInherit } from '@/infra/docker.js';
import { select, confirm, PromptCancelledError } from '@/infra/prompts.js';
import { resolveRemoteImage } from '@/domain/generator.js';
import { LABEL_MANAGED, LABEL_QUICK_RUN } from '@/core/labels.js';
import { REMOTE_VARIANTS, VARIANT_LABELS, parseVariant, type RemoteVariant } from '@/core/types.js';
import { parseCommonFlags, type CommonFlags } from '@/infra/parse.js';
import type { Command } from '@/commands/command.js';
export interface QuickRunFlags extends CommonFlags {
  variant?: RemoteVariant;
  volume?: string;
  name?: string;
  port?: number;
  registry?: string;
}

export function parseQuickRunFlags(argv: string[]): QuickRunFlags {
  const { flags, remaining } = parseCommonFlags(argv);
  const result: QuickRunFlags = { ...flags };
  for (let i = 0; i < remaining.length; i++) {
    const a = remaining[i];
    const next = () => remaining[++i];
    switch (a) {
      case '--variant':
        result.variant = parseVariant(next());
        break;
      case '--volume':
        result.volume = next();
        break;
      case '--name':
        result.name = next();
        break;
      case '--port': {
        const n = Number(next());
        if (!Number.isInteger(n) || n < 0 || n > 65535) throw new Error('Invalid --port value');
        result.port = n;
        break;
      }
      case '--registry':
        result.registry = next();
        break;
      default:
        throw new Error(`Unknown flag for run: ${a}`);
    }
  }
  return result;
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
