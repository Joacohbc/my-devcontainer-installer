import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { loadConfig } from '@/config.js';
import { confirm } from '@/prompts.js';
import { resolveProjectComposeFile } from '@/lifecycle.js';
import { sanitizeDockerName } from '@/validators.js';
import * as path from 'path';

export interface DownFlags {
  yes: boolean;
  volumes: boolean;
  help: boolean;
  interactive: boolean;
}

export function parseDownFlags(argv: string[]): DownFlags {
  const flags: DownFlags = { yes: false, volumes: false, help: false, interactive: true };
  for (const a of argv) {
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '-y':
      case '--yes':
        flags.yes = true;
        break;
      case '-v':
      case '--volumes':
        flags.volumes = true;
        break;
      case '--no-interactive':
        flags.interactive = false;
        break;
      default:
        throw new Error(`Unknown flag for down: ${a}`);
    }
  }
  return flags;
}

export function downHelp(): string {
  return `devcontainer-cli down — stop and remove project containers

Usage:
  devcontainer-cli down [flags]

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml down
Add -v/--volumes to also remove named volumes (deletes data).

Flags:
  -v, --volumes      Also remove named volumes (docker compose down -v)
  -y, --yes          Skip the volume prompt (keeps volumes unless -v is given)
  --no-interactive   Non-interactive mode
  -h, --help         Show this help
`;
}

export async function runDown(argv: string[]): Promise<void> {
  const flags = parseDownFlags(argv);
  if (flags.help) { console.log(downHelp()); return; }

  const cwd = process.cwd();
  const config = loadConfig(cwd);
  const workspace = config?.workspace ?? sanitizeDockerName(path.basename(cwd));
  const composeFile = resolveProjectComposeFile(cwd);

  let removeVolumes = flags.volumes;
  if (!removeVolumes && !flags.yes && flags.interactive) {
    removeVolumes = await confirm(
      'downVolumes',
      `Also remove named volumes for '${workspace}'? This deletes their data.`,
      false,
    );
  }

  const args = ['compose', '-f', composeFile, 'down'];
  if (removeVolumes) args.push('-v');

  console.log(chalk.yellow(`\nBringing down '${workspace}'${removeVolumes ? ' (with volumes)' : ''}...\n`));
  const r = spawnSync('docker', args, { stdio: 'inherit' });
  if ((r.status ?? -1) !== 0) throw new Error('docker compose down failed.');
  console.log(chalk.green.bold('\nDone.\n'));
}
