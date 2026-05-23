import chalk from 'chalk';
import {
  loadGlobalConfig,
  saveGlobalConfig,
  globalConfigPath,
  REGISTRY_DEFAULT,
} from './global-config.js';

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

export async function runConfigCmd(argv: string[]): Promise<void> {
  if (argv.length === 0 || argv[0] === '-h' || argv[0] === '--help') {
    console.log(configCmdHelp());
    return;
  }
  const key = argv[0];
  if (key !== 'registry') {
    throw new Error(`Unknown config key: ${key}. Supported: registry`);
  }
  const rest = argv.slice(1);
  if (rest.length === 0) {
    const cfg = loadGlobalConfig();
    const effective = cfg.registry ?? REGISTRY_DEFAULT;
    console.log(effective);
    if (!cfg.registry) {
      console.log(chalk.gray('(default — not yet customized)'));
    }
    return;
  }
  if (rest[0] === '--unset') {
    const cfg = loadGlobalConfig();
    delete cfg.registry;
    saveGlobalConfig(cfg);
    console.log(chalk.green(`✓ Unset registry (will use default: ${REGISTRY_DEFAULT}).`));
    return;
  }
  const value = rest[0];
  if (!value) throw new Error('Missing value for registry');
  const cfg = loadGlobalConfig();
  cfg.registry = value;
  saveGlobalConfig(cfg);
  console.log(chalk.green(`✓ Set registry = ${cfg.registry} (in ${globalConfigPath()})`));
}
