import type { Command } from '@/commands/command.js';
import { generateCommand } from '@/commands/generate.js';
import { setupSshCommand } from '@/commands/setup-ssh.js';
import { portForwardCommand } from '@/commands/port-forward.js';
import { quickRunCommand } from '@/commands/quick-run.js';
import { downCommand } from '@/commands/down.js';
import { destroyCommand } from '@/commands/destroy.js';
import { pruneCommand } from '@/commands/prune.js';
import { lifecycleCommands } from '@/commands/lifecycle.js';
import { updateCommand } from '@/commands/update-images.js';
import { upgradeCliCommand } from '@/commands/self-update.js';
import { configCommand } from '@/commands/config-cmd.js';
import { cleanupTipsCommand } from '@/commands/cleanup-instructions.js';
import { completionCommand, completeHiddenCommand } from '@/commands/completion-cmd.js';

// The default command that runs when no subcommand token is given.
export const defaultCommand: Command = generateCommand;

let cached: Command[] | null = null;

// Single source of truth for every named (sub)command. Both the dispatcher in
// index.ts and the shell-completion engine read from this list. Built lazily so
// the command objects (which form an import cycle through completion) are only
// referenced after every module has finished initializing.
export function getCommands(): Command[] {
  if (!cached) {
    cached = [
      completeHiddenCommand,
      completionCommand,
      setupSshCommand,
      portForwardCommand,
      quickRunCommand,
      downCommand,
      destroyCommand,
      pruneCommand,
      ...lifecycleCommands,
      cleanupTipsCommand,
      updateCommand,
      upgradeCliCommand,
      configCommand,
    ];
  }
  return cached;
}

// Names of user-facing commands (hidden ones excluded) used to offer
// completion candidates for the first token.
export function completableCommandNames(): string[] {
  return getCommands()
    .filter((c) => !c.hidden && c.name)
    .map((c) => c.name);
}
