import chalk from 'chalk';
import { LABEL_MANAGED, LABEL_PROJECT, projectId } from '@/labels.js';
import type { DevcontainerConfig } from '@/types.js';
import { defaultConfig, loadConfig } from '@/config.js';
import { resolveWorkspace } from '@/project.js';
import type { Command } from '@/commands/command.js';

export function printCleanupInstructions(config: DevcontainerConfig): void {
  const project = projectId(config);
  const bar = chalk.gray('─'.repeat(64));
  const prompt = chalk.gray('   $ ');
  const allFilter = `--filter "label=${LABEL_MANAGED}=true"`;
  const projFilter = `--filter "label=${LABEL_PROJECT}=${project}"`;

  const out = [
    chalk.cyan.bold('Cleanup / update — managed by label'),
    bar,
    chalk.gray(`Every image, container, volume and network created by this CLI is tagged with:`),
    chalk.gray(`  ${LABEL_MANAGED}=true`),
    chalk.gray(`  ${LABEL_PROJECT}=${project}`),
    '',
    chalk.bold('List resources of THIS project:'),
    prompt + `docker ps -a ${projFilter}`,
    prompt + `docker images ${projFilter}`,
    prompt + `docker volume ls ${projFilter}`,
    '',
    chalk.bold('List resources of ALL projects managed by this CLI:'),
    prompt + `docker ps -a ${allFilter}`,
    prompt + `docker images ${allFilter}`,
    '',
    chalk.bold('Stop + remove THIS project (containers, network, volumes):'),
    prompt + 'docker compose down -v',
    '',
    chalk.bold('Purge dangling/unused images of THIS project:'),
    prompt + `docker image prune -a ${projFilter} -f`,
    '',
    chalk.bold('Purge ALL CLI-managed images (every project):'),
    prompt + `docker image prune -a ${allFilter} -f`,
    '',
    chalk.bold('Update (rebuild without cache + recreate):'),
    prompt + 'docker compose build --no-cache',
    prompt + 'docker compose up -d --force-recreate',
    bar,
    '',
  ].join('\n');

  console.log(out);
}

export interface CleanupTipsFlags {
  help: boolean;
}

export function parseCleanupTipsFlags(argv: string[]): CleanupTipsFlags {
  const flags: CleanupTipsFlags = { help: false };
  for (const a of argv) {
    if (a === '-h' || a === '--help') flags.help = true;
    else throw new Error(`Unknown flag for cleanup-tips: ${a}`);
  }
  return flags;
}

export function cleanupTipsHelp(): string {
  return `devcontainer-cli cleanup-tips — show docker cleanup commands for this project

Usage:
  devcontainer-cli cleanup-tips [flags]

Flags:
  -h, --help         Show this help
`;
}

export async function runCleanupTips(argv: string[]): Promise<void> {
  const flags = parseCleanupTipsFlags(argv);
  if (flags.help) {
    console.log(cleanupTipsHelp());
    return;
  }
  const config = loadConfig(process.cwd()) ?? defaultConfig();
  if (!config.workspace) {
    config.workspace = resolveWorkspace(process.cwd(), config);
  }
  printCleanupInstructions(config);
}

export const cleanupTipsCommand: Command<CleanupTipsFlags> = {
  name: 'cleanup-tips',
  summary: 'Show docker cleanup commands for this project',
  parse: parseCleanupTipsFlags,
  help: cleanupTipsHelp,
  run: runCleanupTips,
};
