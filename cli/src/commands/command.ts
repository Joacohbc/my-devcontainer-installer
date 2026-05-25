import { validateAssetReferences } from '@/infra/preflight.js';

// Reusable shape every CLI command (and subcommand) implements. It captures the
// four things that previously lived as a loose convention of free functions per
// file: argument parsing, help text, the run entry point, and metadata used by
// the dispatcher and shell completion.
export interface Command<F = unknown> {
  // Invocation token, e.g. 'down'. Empty for the default command that runs when
  // no subcommand is given.
  name: string;
  // One-line description shown in the root help listing.
  summary: string;
  aliases?: string[];
  // Excluded from help and completion (e.g. the internal '__complete' command).
  hidden?: boolean;
  // Static asset files (relative to assets/) validated before run() so a
  // missing reference fails fast.
  requiredAssets?: string[];
  // Nested subcommands, e.g. 'config' → 'registry'.
  subcommands?: Command[];
  parse(argv: string[]): F;
  help(): string;
  run(argv: string[]): Promise<void>;
}

export function findCommand(commands: Command[], token: string): Command | undefined {
  return commands.find((c) => c.name === token || c.aliases?.includes(token));
}

// Looks up a command by name and runs it after validating its static asset
// references. Returns false when no command matches the token so the caller can
// fall back (e.g. to the default command).
export async function dispatch(
  commands: Command[],
  token: string,
  argv: string[],
): Promise<boolean> {
  const cmd = findCommand(commands, token);
  if (!cmd) return false;
  await runCommand(cmd, argv);
  return true;
}

export async function runCommand(cmd: Command, argv: string[]): Promise<void> {
  if (cmd.requiredAssets?.length) {
    const missing = validateAssetReferences(cmd.requiredAssets);
    if (missing.length > 0) {
      throw new Error(
        `Missing required asset file(s) for '${cmd.name}': ${missing.join(', ')}. ` +
          'They must be embedded in the binary or present under assets/.',
      );
    }
  }
  await cmd.run(argv);
}
