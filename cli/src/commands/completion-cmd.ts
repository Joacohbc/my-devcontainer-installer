import {
  runCompletion,
  runCompleteHidden,
  parseCompletionArgs,
  type CompletionArgs,
} from '@/completion.js';
import type { Command } from '@/commands/command.js';

export const completionCommand: Command<CompletionArgs> = {
  name: 'completion',
  summary: 'Print shell completion script (bash|zsh)',
  parse: parseCompletionArgs,
  help: () => 'Usage: devcontainer-cli completion <bash|zsh>\n',
  run: runCompletion,
};

export const completeHiddenCommand: Command<string[]> = {
  name: '__complete',
  summary: 'Internal: emit shell completion candidates',
  hidden: true,
  parse: (argv) => argv,
  help: () => '',
  run: runCompleteHidden,
};
