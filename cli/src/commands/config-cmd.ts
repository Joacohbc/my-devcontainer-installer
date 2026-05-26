import chalk from 'chalk';
import {
  loadGlobalConfig,
  saveGlobalConfig,
  globalConfigPath,
  REGISTRY_DEFAULT,
} from '@/domain/global-config.js';
import { findCommand, type Command } from '@/commands/command.js';

export function configCmdHelp(): string {
  return `devcontainer-cli config — read/write global CLI config

Usage:
  devcontainer-cli config registry [<url>]   Get/set the default image registry

Examples:
  devcontainer-cli config registry                       # print current value
  devcontainer-cli config registry ghcr.io/myorg/        # set value
  devcontainer-cli config registry --unset               # remove key

Config file: ${globalConfigPath()}
`;
}

export interface ConfigRegistryArgs {
  value?: string;
  unset: boolean;
}

export function parseConfigRegistryArgs(argv: string[]): ConfigRegistryArgs {
  if (argv[0] === '--unset') return { unset: true };
  return { unset: false, value: argv[0] };
}

async function runConfigRegistry(argv: string[]): Promise<void> {
  const { value, unset } = parseConfigRegistryArgs(argv);
  if (!unset && value === undefined) {
    const cfg = loadGlobalConfig();
    const effective = cfg.registry ?? REGISTRY_DEFAULT;
    console.log(effective);
    if (!cfg.registry) {
      console.log(chalk.gray('(default — not yet customized)'));
    }
    return;
  }
  if (unset) {
    const cfg = loadGlobalConfig();
    delete cfg.registry;
    saveGlobalConfig(cfg);
    console.log(chalk.green(`✓ Unset registry (will use default: ${REGISTRY_DEFAULT}).`));
    return;
  }
  if (!value) throw new Error('Missing value for registry');
  const cfg = loadGlobalConfig();
  cfg.registry = value;
  saveGlobalConfig(cfg);
  console.log(chalk.green(`✓ Set registry = ${cfg.registry} (in ${globalConfigPath()})`));
}

export const configRegistrySubcommand: Command<ConfigRegistryArgs> = {
  name: 'registry',
  summary: 'Get/set the default image registry',
  parse: parseConfigRegistryArgs,
  help: configCmdHelp,
  run: runConfigRegistry,
};

const CONFIG_SUBCOMMANDS: Command[] = [configRegistrySubcommand];

export async function runConfigCmd(argv: string[]): Promise<void> {
  if (argv.length === 0 || argv[0] === '-h' || argv[0] === '--help') {
    console.log(configCmdHelp());
    return;
  }
  const key = argv[0];
  const sub = findCommand(CONFIG_SUBCOMMANDS, key);
  if (!sub) {
    throw new Error(`Unknown config key: ${key}. Supported: registry`);
  }
  await sub.run(argv.slice(1));
}

export const configCommand: Command<string[]> = {
  name: 'config',
  summary: "Read or write global CLI config (e.g. 'config registry <url>')",
  subcommands: CONFIG_SUBCOMMANDS,
  parse: (argv) => argv,
  help: configCmdHelp,
  run: runConfigCmd,
};
