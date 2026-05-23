import * as fs from 'fs';
import * as path from 'path';
import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { loadConfig } from '@/config.js';
import { sanitizeDockerName } from '@/validators.js';

export type LifecycleVerb = 'start' | 'stop' | 'restart';

export interface LifecycleFlags {
  help: boolean;
}

// Resolves the generated compose file for the current project. Shared with `down`.
export function resolveProjectComposeFile(cwd: string): string {
  const config = loadConfig(cwd);
  const workspace = config?.workspace ?? sanitizeDockerName(path.basename(cwd));
  const composeFile = path.join(cwd, `.dc_${workspace}`, 'build', 'docker-compose.yml');
  if (!fs.existsSync(composeFile)) {
    throw new Error(`No compose file found at ${composeFile}. Run 'devcontainer-cli' to generate one first.`);
  }
  return composeFile;
}

export function parseLifecycleFlags(verb: LifecycleVerb, argv: string[]): LifecycleFlags {
  const flags: LifecycleFlags = { help: false };
  for (const a of argv) {
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      default:
        throw new Error(`Unknown flag for ${verb}: ${a}`);
    }
  }
  return flags;
}

export function lifecycleHelp(verb: LifecycleVerb): string {
  return `devcontainer-cli ${verb} — docker compose ${verb} for the current project

Usage:
  devcontainer-cli ${verb} [flags]

Runs: docker compose -f .dc_<workspace>/build/docker-compose.yml ${verb}

Flags:
  -h, --help         Show this help
`;
}

export async function runLifecycle(verb: LifecycleVerb, argv: string[]): Promise<void> {
  const flags = parseLifecycleFlags(verb, argv);
  if (flags.help) { console.log(lifecycleHelp(verb)); return; }

  const composeFile = resolveProjectComposeFile(process.cwd());

  console.log(chalk.yellow(`\nRunning 'docker compose ${verb}'...\n`));
  const r = spawnSync('docker', ['compose', '-f', composeFile, verb], { stdio: 'inherit' });
  if ((r.status ?? -1) !== 0) throw new Error(`docker compose ${verb} failed.`);
  console.log(chalk.green.bold('\nDone.\n'));
}
