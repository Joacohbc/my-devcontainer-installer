import chalk from 'chalk';
import { cleanupStaleUpdate } from '@/commands/self-update.js';
import { PromptCancelledError } from '@/infra/prompts.js';
import { getCommands, defaultCommand } from '@/commands/registry.js';
import { dispatch, runCommand } from '@/commands/command.js';

function installSignalHandlers(): void {
  const cancel = () => {
    if (process.stdin.isTTY && process.stdin.setRawMode) {
      try { process.stdin.setRawMode(false); } catch { /* noop */ }
    }
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  };
  process.once('SIGINT', cancel);
  process.once('SIGTERM', cancel);
  process.on('unhandledRejection', (e: unknown) => {
    const err = e as { code?: string; message?: string } | undefined;
    if (!err) return;
    if (err.code === 'ERR_USE_AFTER_CLOSE') return;
    if (err.message === '' || err.message === 'canceled') return;
    console.error(chalk.red(`\n${err.message ?? String(e)}\n`));
    process.exit(1);
  });
}

async function main() {
  installSignalHandlers();
  cleanupStaleUpdate();

  const argv = process.argv.slice(2);
  const cmd = argv[0];

  if (cmd !== undefined && !cmd.startsWith('-')) {
    const handled = await dispatch(getCommands(), cmd, argv.slice(1));
    if (handled) return;
    console.error(chalk.red(`\nUnknown command: ${cmd}\n`));
    console.log(defaultCommand.help());
    process.exit(1);
  }

  await runCommand(defaultCommand, argv);
}

main().catch((e) => {
  if (e instanceof PromptCancelledError) {
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  }
  console.error(chalk.red(`\n${e.message ?? e}\n`));
  process.exit(1);
});
