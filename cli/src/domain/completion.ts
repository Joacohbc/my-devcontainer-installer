import * as fs from 'fs';
import * as path from 'path';
import { BUILD_MODES, REMOTE_VARIANTS } from '@/core/types.js';
import { composeServices, dockerfileModules } from '@/core/module-registry.js';
import { listManagedContainers } from '@/infra/container-picker.js';
import { getSshAliases } from '@/commands/port-forward.js';
import { completableCommandNames } from '@/commands/registry.js';

export interface CompletionProviders {
  containers: () => string[];
  sshAliases: () => string[];
  modules: () => string[];
  services: () => string[];
}

const SETUP_SSH_MODES = ['local', 'windows', 'remote'] as const;

const ROOT_FLAGS = [
  '--mode',
  '--variant',
  '--registry',
  '--with',
  '--service',
  '--image',
  '--workspace',
  '--config',
  '--no-interactive',
  '--force-prompt',
  '--force',
  '--build',
  '--no-build',
  '-v',
  '--version',
  '-h',
  '--help',
];

interface CommandSpec {
  flags: string[];
  // flags whose value is dynamic/enumerated — keyed by flag, value = provider fn name or static list
  valueFlags: Record<string, (p: CompletionProviders) => string[]>;
  // positional candidate generators, indexed by positional position (0-based among non-flag args after subcommand)
  positionals?: ((p: CompletionProviders, index: number) => string[])[];
}

const COMMAND_SPECS: Record<string, CommandSpec> = {
  'setup-ssh': {
    flags: [
      '--remote',
      '--alias',
      '--key',
      '--port',
      '--mode',
      '--container',
      '--service',
      '--compose-file',
      '-f',
      '--user',
      '-y',
      '--yes',
      '-h',
      '--help',
    ],
    valueFlags: {
      '--mode': () => [...SETUP_SSH_MODES],
      '--container': (p) => p.containers(),
      '--alias': (p) => p.sshAliases(),
      '--service': (p) => p.services(),
    },
  },
  'port-forward': {
    flags: ['--alias', '--service', '--no-interactive', '--non-interactive', '-h', '--help'],
    valueFlags: {
      '--alias': (p) => p.sshAliases(),
      '--service': (p) => p.services(),
    },
  },
  run: {
    flags: [
      '--variant',
      '--name',
      '--volume',
      '--port',
      '--registry',
      '--no-interactive',
      '-h',
      '--help',
    ],
    valueFlags: {
      '--variant': () => [...REMOTE_VARIANTS],
    },
  },
  down: {
    flags: ['-v', '--volumes', '-y', '--yes', '--no-interactive', '-h', '--help'],
    valueFlags: {},
  },
  destroy: {
    flags: ['-y', '--yes', '--no-interactive', '-h', '--help'],
    valueFlags: {},
  },
  prune: {
    flags: ['--all', '-y', '--yes', '--no-interactive', '-h', '--help'],
    valueFlags: {},
  },
  start: { flags: ['-h', '--help'], valueFlags: {} },
  stop: { flags: ['-h', '--help'], valueFlags: {} },
  restart: { flags: ['-h', '--help'], valueFlags: {} },
  update: {
    flags: ['--all', '--pull', '--rebuild', '-h', '--help'],
    valueFlags: {},
  },
  'upgrade-cli': { flags: ['-h', '--help'], valueFlags: {} },
  'cleanup-tips': { flags: ['-h', '--help'], valueFlags: {} },
  config: {
    flags: ['--unset', '-h', '--help'],
    valueFlags: {},
    positionals: [() => ['registry']],
  },
  completion: {
    flags: ['-h', '--help'],
    valueFlags: {},
    positionals: [() => ['bash', 'zsh']],
  },
};

// Root flags that take a value (and the candidates they accept).
const ROOT_VALUE_FLAGS: Record<string, (p: CompletionProviders) => string[]> = {
  '--mode': () => [...BUILD_MODES],
  '--variant': () => [...REMOTE_VARIANTS],
  '--with': (p) => p.modules(),
  '--service': (p) => p.services(),
};

// Flags that take a value but offer no completions (free-form text).
const FREEFORM_VALUE_FLAGS = new Set([
  '--image',
  '--workspace',
  '--name',
  '--volume',
  '--registry',
  '--key',
  '--port',
  '--user',
  '--remote',
  '--compose-file',
  '-f',
  '--config',
]);

function safe<T>(fn: () => T[]): T[] {
  try {
    return fn() ?? [];
  } catch {
    return [];
  }
}

const realProviders: CompletionProviders = {
  containers: () => safe(() => listManagedContainers().map((c) => c.name)),
  sshAliases: () => safe(() => getSshAliases()),
  modules: () => safe(() => dockerfileModules.map((m) => m.id)),
  services: () => safe(() => composeServices.map((s) => s.id)),
};

function findSubcommand(words: string[]): string | undefined {
  // First non-flag token before the cursor word.
  for (let i = 0; i < words.length - 1; i++) {
    const w = words[i];
    if (!w.startsWith('-') && COMMAND_SPECS[w]) return w;
  }
  return undefined;
}

function positionalIndex(words: string[], subcommand: string): number {
  // Count non-flag args after the subcommand, excluding the current (cursor) word.
  let count = 0;
  let seenSub = false;
  for (let i = 0; i < words.length - 1; i++) {
    const w = words[i];
    if (!seenSub) {
      if (w === subcommand) seenSub = true;
      continue;
    }
    if (w.startsWith('-')) {
      // Skip value of value-taking flag.
      const spec = COMMAND_SPECS[subcommand];
      const takesValue =
        spec.valueFlags[w] !== undefined || FREEFORM_VALUE_FLAGS.has(w) || ROOT_VALUE_FLAGS[w] !== undefined;
      if (takesValue) i++;
      continue;
    }
    count++;
  }
  return count;
}

function applyCommaPrefix(current: string, candidates: string[]): string[] {
  // For comma-list flags: if user already typed `a,b,`, complete `a,b,<candidate>`.
  const idx = current.lastIndexOf(',');
  if (idx === -1) return candidates;
  const prefix = current.slice(0, idx + 1);
  return candidates.map((c) => prefix + c);
}

export function completionCandidates(
  words: string[],
  providers: CompletionProviders = realProviders,
): string[] {
  // words = [arg1, arg2, ..., cursorWord]. cursorWord may be ''.
  if (words.length === 0) words = [''];
  const current = words[words.length - 1];
  const previous = words.length >= 2 ? words[words.length - 2] : '';

  const subcommand = findSubcommand(words);

  // Rule 1: previous word is a value-taking flag → return its candidates.
  if (previous.startsWith('-')) {
    if (subcommand) {
      const spec = COMMAND_SPECS[subcommand];
      const dynamic = spec.valueFlags[previous];
      if (dynamic) {
        const cands = dynamic(providers);
        // Comma-list aware (rarely applies to subcommand flags, but harmless).
        return applyCommaPrefix(current, cands);
      }
      if (FREEFORM_VALUE_FLAGS.has(previous)) return [];
      // Unknown subcommand flag with value — fall through to flag suggestions.
    } else {
      const dynamic = ROOT_VALUE_FLAGS[previous];
      if (dynamic) {
        const cands = dynamic(providers);
        // --with and --service support comma-separated lists.
        if (previous === '--with' || previous === '--service') {
          return applyCommaPrefix(current, cands);
        }
        return cands;
      }
      if (FREEFORM_VALUE_FLAGS.has(previous)) return [];
    }
  }

  // Rule 2: no subcommand chosen yet → completing the first token.
  if (!subcommand) {
    // If current starts with `-`, offer root flags.
    if (current.startsWith('-')) {
      return ROOT_FLAGS;
    }
    return [...completableCommandNames(), ...ROOT_FLAGS];
  }

  // Rule 3: have a subcommand → offer its flags + any positional candidates.
  const spec = COMMAND_SPECS[subcommand];
  const out: string[] = [...spec.flags];
  if (spec.positionals) {
    const idx = positionalIndex(words, subcommand);
    const gen = spec.positionals[idx];
    if (gen) out.push(...gen(providers, idx));
  }
  return out;
}

export async function runCompleteHidden(argv: string[]): Promise<void> {
  // A consumer (shell, `head`) may close the pipe early → EPIPE is emitted as an
  // async 'error' event that a try/catch can't reach. Swallow it: a partial
  // candidate list is harmless and crashing would print a stack to stderr.
  process.stdout.on('error', (err: NodeJS.ErrnoException) => {
    if (err.code === 'EPIPE') process.exit(0);
  });
  try {
    const candidates = completionCandidates(argv);
    process.stdout.write(candidates.join('\n') + (candidates.length ? '\n' : ''));
  } catch {
    // Silent: any output on stdout would corrupt shell candidate parsing.
  }
}

export function bashCompletionScript(): string {
  return `# devcontainer-cli bash completion
_devcontainer_cli_complete() {
  local cur="\${COMP_WORDS[COMP_CWORD]}"
  local args=("\${COMP_WORDS[@]:1:COMP_CWORD-1}")
  local candidates
  candidates="$(devcontainer-cli __complete "\${args[@]}" "$cur" 2>/dev/null)"
  COMPREPLY=( $(compgen -W "\${candidates}" -- "$cur") )
}
complete -F _devcontainer_cli_complete devcontainer-cli
`;
}

export function zshCompletionScript(): string {
  return `#compdef devcontainer-cli
_devcontainer_cli() {
  local current="\${words[CURRENT]}"
  local args=("\${(@)words[2,CURRENT-1]}")
  local -a candidates
  candidates=("\${(@f)$(devcontainer-cli __complete "\${args[@]}" "$current" 2>/dev/null)}")
  compadd -- "\${candidates[@]}"
}
_devcontainer_cli "$@"
`;
}

// Completion scripts installed by install.sh live in a `completions/` dir next
// to the binary. They are static wrappers (delegating to `__complete` at
// runtime), so they only need refreshing when their template changes — but
// rewriting them after a self-update is cheap and keeps them in sync. Only
// files that already exist are touched, so a user who opted out of completion
// (SETUP_COMPLETION=0) or installed for only one shell stays untouched.
export function refreshInstalledCompletions(execPath: string): string[] {
  const dir = path.join(path.dirname(execPath), 'completions');
  const targets: Array<{ file: string; gen: () => string }> = [
    { file: '_devcontainer-cli', gen: zshCompletionScript },
    { file: 'devcontainer-cli.bash', gen: bashCompletionScript },
  ];
  const updated: string[] = [];
  for (const t of targets) {
    const p = path.join(dir, t.file);
    if (!fs.existsSync(p)) continue;
    fs.writeFileSync(p, t.gen(), { mode: 0o644 });
    updated.push(p);
  }
  return updated;
}

export interface CompletionArgs {
  shell: 'bash' | 'zsh';
}

export function parseCompletionArgs(argv: string[]): CompletionArgs {
  const shell = argv[0];
  if (shell !== 'bash' && shell !== 'zsh') {
    throw new Error('Usage: devcontainer-cli completion <bash|zsh>');
  }
  return { shell };
}

export async function runCompletion(argv: string[]): Promise<void> {
  const { shell } = parseCompletionArgs(argv);
  const script = shell === 'bash' ? bashCompletionScript() : zshCompletionScript();
  process.stdout.write(script);
}
