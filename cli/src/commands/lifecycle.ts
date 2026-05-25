import chalk from 'chalk';
import { dockerComposeOrThrow } from '@/docker.js';
import { resolveProjectComposeFile } from '@/project.js';
import type { Command } from '@/commands/command.js';

export type LifecycleVerb = 'start' | 'stop' | 'restart';

export interface LifecycleFlags {
  help: boolean;
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
  dockerComposeOrThrow(composeFile, [verb]);
  console.log(chalk.green.bold('\nDone.\n'));
}

export function makeLifecycleCommand(verb: LifecycleVerb): Command<LifecycleFlags> {
  return {
    name: verb,
    summary: `docker compose ${verb} for the current project`,
    parse: (argv) => parseLifecycleFlags(verb, argv),
    help: () => lifecycleHelp(verb),
    run: (argv) => runLifecycle(verb, argv),
  };
}

export const lifecycleCommands: Command<LifecycleFlags>[] = [
  makeLifecycleCommand('start'),
  makeLifecycleCommand('stop'),
  makeLifecycleCommand('restart'),
];
