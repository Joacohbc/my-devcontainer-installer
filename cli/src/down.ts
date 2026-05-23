import * as fs from 'fs';
import * as path from 'path';
import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { loadConfig } from './config.js';
import { confirm, PromptCancelledError } from './prompts.js';
import { sanitizeDockerName } from './validators.js';

export interface DownFlags {
  yes: boolean;
  help: boolean;
  interactive: boolean;
}

export function parseDownFlags(argv: string[]): DownFlags {
  const flags: DownFlags = { yes: false, help: false, interactive: true };
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
  return `devcontainer-cli down — stop and remove project containers + volumes

Usage:
  devcontainer-cli down [flags]

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml down -v

Flags:
  -y, --yes          Skip confirmation prompt
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
  const composeFile = path.join(cwd, `.dc_${workspace}`, 'build', 'docker-compose.yml');

  if (!fs.existsSync(composeFile)) {
    throw new Error(`No compose file found at ${composeFile}. Run 'devcontainer-cli' to generate one first.`);
  }

  if (!flags.yes && flags.interactive) {
    const proceed = await confirm(
      'down',
      `Run 'docker compose down -v' for workspace '${workspace}'? This removes containers and volumes.`,
      false,
    );
    if (!proceed) { console.log(chalk.yellow('Cancelled.')); return; }
  }

  console.log(chalk.yellow(`\n⏳ Bringing down '${workspace}'...\n`));
  const r = spawnSync('docker', ['compose', '-f', composeFile, 'down', '-v'], { stdio: 'inherit' });
  if ((r.status ?? -1) !== 0) throw new Error('docker compose down failed.');
  console.log(chalk.green.bold('\n✅ Done.\n'));
}
