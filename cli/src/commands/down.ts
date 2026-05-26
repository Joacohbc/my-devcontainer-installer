import chalk from 'chalk';
import { loadConfig } from '@/domain/config.js';
import { confirm } from '@/infra/prompts.js';
import { dockerComposeOrThrow } from '@/infra/docker.js';
import { resolveWorkspace, resolveProjectComposeFile } from '@/infra/project.js';
import * as path from 'path';
import { parseCommonFlags, type CommonFlags } from '@/infra/parse.js';
import type { Command } from '@/commands/command.js';

export interface DownFlags extends CommonFlags {
  volumes: boolean;
}

export function parseDownFlags(argv: string[]): DownFlags {
  const { flags, remaining } = parseCommonFlags(argv);
  let volumes = false;
  for (const a of remaining) {
    if (a === '-v' || a === '--volumes') {
      volumes = true;
    } else {
      throw new Error(`Unknown flag for down: ${a}`);
    }
  }
  return { ...flags, volumes };
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
  const workspace = resolveWorkspace(cwd);
  const composeFile = resolveProjectComposeFile(cwd);

  let removeVolumes = flags.volumes;
  if (!removeVolumes && !flags.yes && flags.interactive) {
    removeVolumes = await confirm(
      'downVolumes',
      `Also remove named volumes for '${workspace}'? This deletes their data.`,
      false,
    );
  }

  const args = ['down'];
  if (removeVolumes) args.push('-v');

  console.log(chalk.yellow(`\nBringing down '${workspace}'${removeVolumes ? ' (with volumes)' : ''}...\n`));
  dockerComposeOrThrow(composeFile, args);
  console.log(chalk.green.bold('\nDone.\n'));
}

export const downCommand: Command<DownFlags> = {
  name: 'down',
  summary: 'docker compose down for the current project (-v to drop volumes)',
  parse: parseDownFlags,
  help: downHelp,
  run: runDown,
};
