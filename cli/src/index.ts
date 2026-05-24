import chalk from 'chalk';
import * as path from 'path';
import { runSetupSsh } from '@/setup-ssh.js';
import { cleanupStaleUpdate, runSelfUpdate } from '@/self-update.js';
import { runUpdateImages } from '@/update-images.js';
import { runConfigCmd } from '@/config-cmd.js';
import { runQuickRun } from '@/quick-run.js';
import { runPrune } from '@/prune.js';
import { runPortForward } from '@/port-forward.js';
import { runDown } from '@/down.js';
import { runDestroy } from '@/destroy.js';
import { runLifecycle } from '@/lifecycle.js';
import { printCleanupInstructions } from '@/cleanup-instructions.js';
import { defaultConfig, loadConfig } from '@/config.js';
import { resolveWorkspace } from '@/project.js';
import { PromptCancelledError } from '@/prompts.js';
import { runGenerate } from '@/generate.js';
import { runCompleteHidden, runCompletion } from '@/completion.js';

const COMMANDS: Record<string, (argv: string[]) => Promise<void>> = {
  __complete: runCompleteHidden,
  completion: runCompletion,
  'setup-ssh': runSetupSsh,
  'port-forward': runPortForward,
  run: runQuickRun,
  down: runDown,
  destroy: runDestroy,
  prune: runPrune,
  start: (a) => runLifecycle('start', a),
  stop: (a) => runLifecycle('stop', a),
  restart: (a) => runLifecycle('restart', a),
  update: runUpdateImages,
  'upgrade-cli': runSelfUpdate,
  config: runConfigCmd,
};

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

  if (cmd === 'cleanup-tips') {
    const config = loadConfig(process.cwd()) ?? defaultConfig();
    if (!config.workspace) {
      config.workspace = resolveWorkspace(process.cwd(), config);
    }
    printCleanupInstructions(config);
    return;
  }

  if (cmd !== undefined && !cmd.startsWith('-')) {
    const handler = COMMANDS[cmd];
    if (handler) {
      await handler(argv.slice(1));
      return;
    }
    console.error(chalk.red(`\nUnknown command: ${cmd}\n`));
    const { helpText } = await import('@/cli.js');
    console.log(helpText());
    process.exit(1);
  }

  await runGenerate(argv);
}

main().catch((e) => {
  if (e instanceof PromptCancelledError) {
    console.log(chalk.yellow('\nCancelled.'));
    process.exit(130);
  }
  console.error(chalk.red(`\n${e.message ?? e}\n`));
  process.exit(1);
});
