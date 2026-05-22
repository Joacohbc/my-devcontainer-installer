export interface CliFlags {
  interactive: boolean;
  forcePrompt: boolean;
  build: boolean | null;
  image?: string;
  workspace?: string;
  withModules?: string[];
  services?: string[];
  config?: string;
  force: boolean;
  help: boolean;
  version: boolean;
}

export function parseFlags(argv: string[]): CliFlags {
  const flags: CliFlags = {
    interactive: true,
    forcePrompt: false,
    force: false,
    build: null,
    help: false,
    version: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => argv[++i];
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '-v':
      case '--version':
        flags.version = true;
        break;
      case '--no-interactive':
      case '--non-interactive':
        flags.interactive = false;
        break;
      case '--force-prompt':
        flags.forcePrompt = true;
        break;
      case '--build':
        flags.build = true;
        break;
      case '--no-build':
        flags.build = false;
        break;
      case '--image':
        flags.image = next();
        break;
      case '--workspace':
        flags.workspace = next();
        break;
      case '--with':
        flags.withModules = next().split(',').map((s) => s.trim()).filter(Boolean);
        break;
      case '--service':
      case '--services':
        flags.services = next().split(',').map((s) => s.trim()).filter(Boolean);
        break;
      case '--config':
        flags.config = next();
        break;
      case '--force':
        flags.force = true;
        break;
      default:
        if (a.startsWith('--')) {
          throw new Error(`Unknown flag: ${a}`);
        }
    }
  }
  return flags;
}

export function helpText(): string {
  return `devcontainer CLI — generate Dockerfile + docker-compose.yml

Usage:
  cli [flags]
  cli standalone [flags]    Generate Dockerfile + docker build/run (no Compose)
  cli setup-ssh [flags]     Run automated SSH setup (see: cli setup-ssh --help)
  cli cleanup-tips [flags]  Show docker cleanup commands for this project
  cli update [flags]        Replace this binary with the latest GitHub release

Flags:
  --with <ids>          Comma-separated dockerfile modules (e.g. nodejs,java,dod)
  --service <ids>       Comma-separated compose services (e.g. mongo,postgres,tunnel)
  --image <name>        Image name (default: devcontainer-ssh:local)
  --workspace <name>    Workspace name (default: current dir name, used for .dc_<name>/)
  --config <path>       Path to devcontainer.config.json
  --no-interactive      Fail if any value is missing instead of prompting
  --force-prompt        Prompt even if config file exists
  --force               Overwrite existing files without prompting
  --build / --no-build  Run 'docker compose build' after generating (default: ask)
  -v, --version         Print the CLI version
  -h, --help            Show this help
`;
}
