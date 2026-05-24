import { BUILD_MODES, REMOTE_VARIANTS, parseVariant, type BuildMode, type RemoteVariant } from '@/types.js';

export interface CliFlags {
  interactive: boolean;
  forcePrompt: boolean;
  build: boolean | null;
  image?: string;
  workspace?: string;
  withModules?: string[];
  services?: string[];
  force: boolean;
  help: boolean;
  version: boolean;
  mode?: BuildMode;
  variant?: RemoteVariant;
  registry?: string;
}

function parseMode(v: string): BuildMode {
  if ((BUILD_MODES as readonly string[]).includes(v)) return v as BuildMode;
  throw new Error(`Invalid --mode: ${v}. Expected one of: ${BUILD_MODES.join(', ')}`);
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
      case '--force':
        flags.force = true;
        break;
      case '--mode':
        flags.mode = parseMode(next());
        break;
      case '--variant':
        flags.variant = parseVariant(next());
        break;
      case '--registry':
        flags.registry = next();
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
  cli setup-ssh [flags]     Run automated SSH setup (see: cli setup-ssh --help)
  cli port-forward [flags]  Forward host port to a container port using SSH
  cli run [flags]           Spin up a remote image container without project files
  cli start [flags]         docker compose start for the current project
  cli stop [flags]          docker compose stop for the current project
  cli restart [flags]       docker compose restart for the current project
  cli down [flags]          docker compose down for the current project (-v to drop volumes)
  cli destroy [flags]       down -v + delete .dc_<workspace>/ and config (irreversible)
  cli prune [flags]         Remove orphan devcontainer-cli/* images
  cli cleanup-tips [flags]  Show docker cleanup commands for this project
  cli update [flags]        Update container images for this project / all projects
  cli upgrade-cli [flags]   Replace this binary with the latest GitHub release
  cli config <key> [<val>]  Read or write global CLI config (e.g. 'config registry <url>')

Flags:
  --mode <name>         Build mode: local-cached (default), remote
  --variant <name>      Remote variant: ssh, nodejs, bun, java-temurin, python, go, node-go, node-python, node-java-temurin, bun-go, bun-python, bun-java-temurin
  --registry <url>      Container registry prefix for remote images (overrides global)
  --with <ids>          Comma-separated dockerfile modules (e.g. nodejs,java,dod)
  --service <ids>       Comma-separated compose services (e.g. mongo,postgres,tunnel)
  --image <name>        Image name (default: derived from fingerprint for local-cached)
  --workspace <name>    Workspace name (default: current dir name, used for .dc_<name>/)
  --no-interactive      Fail if any value is missing instead of prompting
  --force-prompt        Prompt even if config file exists
  --force               Overwrite existing files without prompting
  --build / --no-build  Run 'docker compose build' after generating (default: ask)
  -v, --version         Print the CLI version
  -h, --help            Show this help
`;
}
