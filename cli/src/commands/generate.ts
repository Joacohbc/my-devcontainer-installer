import { parseFlags, helpText, type CliFlags } from '@/cli.js';
import { runGenerate } from '@/generate.js';
import type { Command } from '@/commands/command.js';

// The default command: runs when the CLI is invoked with no recognised
// subcommand. It generates the Dockerfile + docker-compose.yml for the project.
export const generateCommand: Command<CliFlags> = {
  name: '',
  summary: 'generate Dockerfile + docker-compose.yml',
  parse: parseFlags,
  help: helpText,
  run: runGenerate,
};
