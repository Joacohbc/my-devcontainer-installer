import * as fs from 'fs';
import * as path from 'path';
import { dockerComposeOrThrow } from '@/docker.js';
import chalk from 'chalk';
import { loadConfig, configPath } from '@/config.js';
import { confirm } from '@/prompts.js';
import { removeEntry } from '@/image-registry.js';
import { resolveWorkspace, projectPaths } from '@/project.js';

import { parseCommonFlags, helpBlock } from '@/parse.js';
import type { Command } from '@/commands/command.js';

export interface DestroyFlags {
  yes: boolean;
  help: boolean;
  interactive: boolean;
}

export function parseDestroyFlags(argv: string[]): DestroyFlags {
  const { flags, remaining } = parseCommonFlags(argv);
  if (remaining.length > 0) {
    throw new Error(`Unknown flag for destroy: ${remaining[0]}`);
  }
  return flags;
}

export function destroyHelp(): string {
  return helpBlock(
    'devcontainer-cli destroy',
    'tear down everything for the current project',
    'devcontainer-cli destroy [flags]\n\nRuns \'docker compose down -v\' (containers + volumes), then deletes the generated\n.dc_<workspace>/ directory and devcontainer.config.json. This is irreversible.',
    [
      { name: '-y, --yes', description: 'Skip confirmation prompt' },
      { name: '--no-interactive', description: 'Non-interactive mode' },
      { name: '-h, --help', description: 'Show this help' },
    ]
  );
}

export async function runDestroy(argv: string[]): Promise<void> {
  const flags = parseDestroyFlags(argv);
  if (flags.help) { console.log(destroyHelp()); return; }

  const cwd = process.cwd();
  const workspace = resolveWorkspace(cwd);
  const paths = projectPaths(cwd, workspace);
  const projectDir = paths.projectDir;
  const composeFile = paths.composeFile;
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
    console.log(chalk.yellow(`\nBringing down '${workspace}' (with volumes)...\n`));
    dockerComposeOrThrow(composeFile, ['down', '-v']);
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

  console.log(chalk.green.bold('\nDestroyed.\n'));
}

export const destroyCommand: Command<DestroyFlags> = {
  name: 'destroy',
  summary: 'down -v + delete .dc_<workspace>/ and config (irreversible)',
  parse: parseDestroyFlags,
  help: destroyHelp,
  run: runDestroy,
};
