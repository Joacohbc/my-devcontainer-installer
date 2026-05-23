import * as fs from 'fs';
import * as path from 'path';
import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { loadConfig, configPath } from './config.js';
import { confirm } from './prompts.js';
import { removeEntry } from './image-registry.js';
import { sanitizeDockerName } from './validators.js';

export interface DestroyFlags {
  yes: boolean;
  help: boolean;
  interactive: boolean;
}

export function parseDestroyFlags(argv: string[]): DestroyFlags {
  const flags: DestroyFlags = { yes: false, help: false, interactive: true };
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
        throw new Error(`Unknown flag for destroy: ${a}`);
    }
  }
  return flags;
}

export function destroyHelp(): string {
  return `devcontainer-cli destroy — tear down everything for the current project

Usage:
  devcontainer-cli destroy [flags]

Runs 'docker compose down -v' (containers + volumes), then deletes the generated
.dc_<workspace>/ directory and devcontainer.config.json. This is irreversible.

Flags:
  -y, --yes          Skip the confirmation prompt
  --no-interactive   Non-interactive mode (requires -y to proceed)
  -h, --help         Show this help
`;
}

export async function runDestroy(argv: string[]): Promise<void> {
  const flags = parseDestroyFlags(argv);
  if (flags.help) { console.log(destroyHelp()); return; }

  const cwd = process.cwd();
  const config = loadConfig(cwd);
  const workspace = config?.workspace ?? sanitizeDockerName(path.basename(cwd));
  const projectDir = path.join(cwd, `.dc_${workspace}`);
  const composeFile = path.join(projectDir, 'build', 'docker-compose.yml');
  const cfgPath = configPath(cwd);

  if (!flags.yes) {
    if (!flags.interactive) {
      throw new Error("destroy is irreversible; pass --yes to confirm in non-interactive mode.");
    }
    const proceed = await confirm(
      'destroy',
      `Destroy '${workspace}'? Removes containers, volumes, .dc_${workspace}/ and devcontainer.config.json.`,
      false,
    );
    if (!proceed) { console.log(chalk.yellow('Cancelled.')); return; }
  }

  if (fs.existsSync(composeFile)) {
    console.log(chalk.yellow(`\n⏳ Bringing down '${workspace}' (with volumes)...\n`));
    const r = spawnSync('docker', ['compose', '-f', composeFile, 'down', '-v'], { stdio: 'inherit' });
    if ((r.status ?? -1) !== 0) throw new Error('docker compose down failed.');
  } else {
    console.log(chalk.gray(`No compose file at ${composeFile}; skipping 'docker compose down'.`));
  }

  if (fs.existsSync(projectDir)) {
    fs.rmSync(projectDir, { recursive: true, force: true });
    console.log(chalk.gray(`Removed ${projectDir}`));
  }
  if (fs.existsSync(cfgPath)) {
    fs.rmSync(cfgPath, { force: true });
    console.log(chalk.gray(`Removed ${cfgPath}`));
  }
  removeEntry(cwd);

  console.log(chalk.green.bold('\n✅ Destroyed.\n'));
}
