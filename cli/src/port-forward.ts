import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { spawn } from 'child_process';
import chalk from 'chalk';
import { input, select } from './prompts.js';

export interface PortForwardConfig {
  portMapping?: string;
  alias?: string;
  service?: string;
  help: boolean;
  interactive: boolean;
}

export interface ParsedPortMapping {
  localPort: number;
  targetHost: string;
  containerPort: number;
}

export function parsePortForwardFlags(argv: string[]): PortForwardConfig {
  const flags: PortForwardConfig = {
    help: false,
    interactive: true,
  };

  const positional: string[] = [];

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => {
      if (i + 1 >= argv.length) {
        throw new Error(`Missing value for flag: ${a}`);
      }
      return argv[++i];
    };
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '--alias':
        flags.alias = next();
        break;
      case '--service':
        flags.service = next();
        break;
      case '--no-interactive':
      case '--non-interactive':
        flags.interactive = false;
        break;
      default:
        if (a.startsWith('-')) {
          throw new Error(`Unknown flag: ${a}`);
        }
        positional.push(a);
    }
  }

  if (positional.length > 1) {
    throw new Error(`Duplicate or invalid arguments: ${positional.join(', ')}`);
  } else if (positional.length === 1) {
    flags.portMapping = positional[0];
  }

  return flags;
}

export function parsePortMapping(mapping: string, defaultService?: string): ParsedPortMapping {
  const parts = mapping.split(':').map((s) => s.trim());

  if (parts.length === 1) {
    const port = parseInt(parts[0], 10);
    if (isNaN(port) || port <= 0 || port > 65535) {
      throw new Error(`Invalid port: ${parts[0]}`);
    }
    return {
      localPort: port,
      targetHost: defaultService || 'localhost',
      containerPort: port,
    };
  } else if (parts.length === 2) {
    const local = parseInt(parts[0], 10);
    const container = parseInt(parts[1], 10);
    if (isNaN(local) || local <= 0 || local > 65535) {
      throw new Error(`Invalid local port: ${parts[0]}`);
    }
    if (isNaN(container) || container <= 0 || container > 65535) {
      throw new Error(`Invalid container port: ${parts[1]}`);
    }
    return {
      localPort: local,
      targetHost: defaultService || 'localhost',
      containerPort: container,
    };
  } else if (parts.length === 3) {
    const local = parseInt(parts[0], 10);
    const targetHost = parts[1];
    const container = parseInt(parts[2], 10);

    if (isNaN(local) || local <= 0 || local > 65535) {
      throw new Error(`Invalid local port: ${parts[0]}`);
    }
    if (!targetHost) {
      throw new Error(`Invalid target host: empty string`);
    }
    if (isNaN(container) || container <= 0 || container > 65535) {
      throw new Error(`Invalid container port: ${parts[2]}`);
    }

    if (defaultService && defaultService !== targetHost) {
      throw new Error(`Conflicting target hosts: mapping specifies '${targetHost}' but --service is '${defaultService}'`);
    }

    return {
      localPort: local,
      targetHost,
      containerPort: container,
    };
  } else {
    throw new Error(`Invalid port mapping format: ${mapping}`);
  }
}

export function parseSshConfigContent(content: string): string[] {
  const hosts: string[] = [];
  const lines = content.split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;

    const match = line.match(/^[ \t]*[Hh][Oo][Ss][Tt][ \t]+([^#]+)/);
    if (match) {
      const hostPart = match[1].trim();
      const parts = hostPart.split(/[ \t]+/);
      for (const p of parts) {
        const h = p.trim();
        if (h && !h.includes('*') && !h.includes('?')) {
          if (!hosts.includes(h)) {
            hosts.push(h);
          }
        }
      }
    }
  }
  return hosts;
}

export function getSshAliases(configPath?: string): string[] {
  const p = configPath || path.join(os.homedir(), '.ssh', 'config');
  if (!fs.existsSync(p)) return [];
  try {
    const content = fs.readFileSync(p, 'utf8');
    return parseSshConfigContent(content);
  } catch (e) {
    return [];
  }
}

export function portForwardHelp(): string {
  return `devcontainer-cli port-forward — forward host port to a container port using SSH

Usage:
  devcontainer-cli port-forward [port_mapping] [flags]

Examples:
  devcontainer-cli port-forward 3000
  devcontainer-cli port-forward 8080:80
  devcontainer-cli port-forward 5432:postgres:5432
  devcontainer-cli port-forward 5432 --service postgres
  devcontainer-cli port-forward 3000 --alias my-custom-host

Flags:
  --alias <name>       SSH host alias to use (bypasses auto-discovery)
  --service <name>     Compose service to map port to (default: localhost)
  --no-interactive     Disable interactive prompts (fail on missing config)
  --non-interactive    Disable interactive prompts (fail on missing config)
  -h, --help           Show this help
`;
}

export async function runPortForward(argv: string[]): Promise<void> {
  const flags = parsePortForwardFlags(argv);
  if (flags.help) {
    console.log(portForwardHelp());
    return;
  }

  let mappingStr = flags.portMapping;
  if (!mappingStr) {
    if (!flags.interactive) {
      throw new Error('Port mapping is required in non-interactive mode.');
    }
    mappingStr = await input(
      'portMapping',
      'Enter port mapping (e.g. 3000, 8080:80, 5432:postgres:5432):',
      undefined,
      (v) => {
        try {
          parsePortMapping(v, flags.service);
          return true;
        } catch (e) {
          return (e as Error).message;
        }
      }
    );
  }

  const mapping = parsePortMapping(mappingStr, flags.service);

  let alias = flags.alias;
  if (!alias) {
    const aliases = getSshAliases();
    if (aliases.length === 1) {
      console.log(chalk.cyan(`Detected a single SSH alias in config: '${aliases[0]}'. Using it.`));
      alias = aliases[0];
    } else if (aliases.length > 1) {
      if (!flags.interactive) {
        throw new Error('Multiple SSH aliases found in config. Specify --alias or run interactively.');
      }
      alias = await select(
        'alias',
        'Select SSH alias to use:',
        aliases.map((a) => ({ name: a, message: a }))
      );
    } else {
      if (!flags.interactive) {
        throw new Error('No SSH aliases found in config. Specify --alias or run interactively.');
      }
      alias = await input(
        'alias',
        'No SSH aliases auto-discovered. Enter SSH alias manually (e.g. devcontainer):',
        undefined,
        (v) => (v.trim() ? true : 'SSH alias cannot be empty.')
      );
    }
  }

  const { localPort, targetHost, containerPort } = mapping;

  console.log(chalk.cyan(`\n🔑 Establishing SSH Tunnel mapping local port ${localPort} to ${targetHost}:${containerPort} on alias '${alias}'...`));
  console.log(chalk.yellow(`Command: ssh -N -L ${localPort}:${targetHost}:${containerPort} ${alias}`));
  console.log(chalk.green(`Press Ctrl+C to terminate the port forwarding session.\n`));

  return new Promise<void>((resolve, reject) => {
    const child = spawn('ssh', ['-N', '-L', `${localPort}:${targetHost}:${containerPort}`, alias], {
      stdio: 'inherit',
    });

    child.on('error', (err) => {
      reject(new Error(`Failed to spawn SSH process: ${err.message}`));
    });

    child.on('close', (code) => {
      if (code !== 0 && code !== null) {
        reject(new Error(`SSH tunnel closed with exit code ${code}`));
      } else {
        console.log(chalk.green(`\n✅ SSH tunnel closed.\n`));
        resolve();
      }
    });
  });
}
