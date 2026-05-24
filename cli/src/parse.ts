export interface CommonFlags {
  help: boolean;
  yes: boolean;
  interactive: boolean;
}

export function parseCommonFlags(argv: string[]): {
  flags: CommonFlags;
  remaining: string[];
} {
  const flags: CommonFlags = {
    help: false,
    yes: false,
    interactive: true,
  };
  const remaining: string[] = [];

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '-y':
      case '--yes':
        flags.yes = true;
        break;
      case '--no-interactive':
        flags.interactive = false;
        break;
      default:
        remaining.push(a);
    }
  }

  return { flags, remaining };
}

export interface HelpFlagDoc {
  name: string;
  description: string;
}

export function helpBlock(
  commandName: string,
  description: string,
  usage: string,
  flags: HelpFlagDoc[],
): string {
  const lines = [
    `${commandName} — ${description}`,
    '',
    'Usage:',
    `  ${usage}`,
    '',
    'Flags:',
  ];

  for (const f of flags) {
    lines.push(`  ${f.name.padEnd(18)} ${f.description}`);
  }

  return lines.join('\n') + '\n';
}
