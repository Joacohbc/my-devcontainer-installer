import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { spawn, type ChildProcess } from 'child_process';
import chalk from 'chalk';
import { input, select, confirm } from '@/prompts.js';
import {
  pickManagedContainer,
  containerWorkspace,
  listAllContainers,
  statusLabel,
  type DockerContainer,
} from '@/container-picker.js';

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

export interface PortPair {
  localPort: number;
  containerPort: number;
}

interface PlannedTunnel {
  localPort: number;
  containerPort: number;
  targetHost: string;
  alias: string;
  containerName: string;
  isDevcontainer: boolean;
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

function parsePort(value: string, label: string): number {
  const n = parseInt(value, 10);
  if (isNaN(n) || n <= 0 || n > 65535) {
    throw new Error(`Invalid ${label}: ${value}`);
  }
  return n;
}

export function parsePortPair(item: string): PortPair {
  const parts = item.split(':').map((s) => s.trim());
  if (parts.length === 1) {
    const port = parsePort(parts[0], 'port');
    return { localPort: port, containerPort: port };
  }
  if (parts.length === 2) {
    return {
      localPort: parsePort(parts[0], 'local port'),
      containerPort: parsePort(parts[1], 'container port'),
    };
  }
  throw new Error(`Invalid port mapping: ${item} (expected 'port' or 'local:container')`);
}

export function parsePortsList(value: string): PortPair[] {
  const items = value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  if (items.length === 0) {
    throw new Error('No ports specified.');
  }
  return items.map(parsePortPair);
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
  return `devcontainer-cli port-forward — forward host ports to container ports using SSH

Usage:
  devcontainer-cli port-forward [port_mapping] [flags]

Run with no port_mapping for an interactive session: pick any running container
(devcontainers are marked '(devcontainer)'), enter one or more ports, and repeat
for other containers. All tunnels are opened in parallel after you confirm.
Non-devcontainer targets are reached through a devcontainer SSH jump host.

Examples:
  devcontainer-cli port-forward                 # interactive multi-container picker
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

/** SSH alias a container exposes itself under, or null when none is configured. */
function ownSshAlias(containerName: string, aliases: string[]): string | null {
  const candidate = containerWorkspace(containerName) ?? containerName;
  return aliases.includes(candidate) ? candidate : null;
}

/** SSH aliases of running devcontainers, usable as jump hosts. */
function jumpHostAliases(containers: DockerContainer[], aliases: string[]): string[] {
  const result: string[] = [];
  for (const c of containers) {
    if (!c.managed) continue;
    const a = ownSshAlias(c.name, aliases);
    if (a && !result.includes(a)) result.push(a);
  }
  return result;
}

async function pickContainerFromList(
  containers: DockerContainer[],
  prompt: string,
): Promise<DockerContainer> {
  const maxName = Math.max(...containers.map((c) => c.name.length));
  const maxImage = Math.max(...containers.map((c) => c.image.length));
  const chosen = await select(
    'container',
    prompt,
    containers.map((c) => ({
      name: c.name,
      message: `${c.name.padEnd(maxName)}  ${
        c.managed ? chalk.green('(devcontainer)') : chalk.gray('              ')
      }  ${chalk.gray(c.image.padEnd(maxImage))}  ${statusLabel(c)}`,
    })),
  );
  return containers.find((c) => c.name === chosen)!;
}

/**
 * Resolve which SSH alias to tunnel through for a target container, plus the
 * host name the tunnel should reach on the far side. Devcontainers with their
 * own alias are reached directly (targetHost = localhost); everything else is
 * reached by container name through a devcontainer jump host.
 */
async function resolveTarget(
  container: DockerContainer,
  aliases: string[],
  containers: DockerContainer[],
  state: { jumpHost?: string },
  interactive: boolean,
): Promise<{ alias: string; targetHost: string }> {
  if (container.managed) {
    const own = ownSshAlias(container.name, aliases);
    if (own) return { alias: own, targetHost: 'localhost' };
  }

  if (!state.jumpHost) {
    const candidates = jumpHostAliases(containers, aliases);
    if (candidates.length === 1) {
      state.jumpHost = candidates[0];
      console.log(chalk.cyan(`Using SSH jump host '${state.jumpHost}'.`));
    } else if (candidates.length > 1) {
      if (!interactive) {
        throw new Error('Multiple SSH jump hosts available; specify --alias.');
      }
      state.jumpHost = await select(
        'jumpHost',
        'Select the devcontainer SSH host to tunnel through:',
        candidates.map((a) => ({ name: a, message: a })),
      );
    } else {
      if (!interactive) {
        throw new Error(
          `No devcontainer SSH alias found to tunnel through '${container.name}'. Run 'setup-ssh' first or specify --alias.`,
        );
      }
      state.jumpHost = await input(
        'jumpHost',
        `No devcontainer SSH alias found to reach '${container.name}'. Enter SSH alias to tunnel through:`,
        undefined,
        (v) => (v.trim() ? true : 'SSH alias cannot be empty.'),
      );
    }
  }

  return { alias: state.jumpHost, targetHost: container.name };
}

async function buildTunnelsInteractive(flags: PortForwardConfig): Promise<PlannedTunnel[]> {
  const containers = listAllContainers();
  if (containers.length === 0) {
    throw new Error("No running containers found. Start a container with 'devcontainer-cli' first.");
  }

  const aliases = getSshAliases();
  const tunnels: PlannedTunnel[] = [];
  const state: { jumpHost?: string } = { jumpHost: flags.alias };

  for (;;) {
    const container =
      containers.length === 1
        ? containers[0]
        : await pickContainerFromList(containers, 'Select a container to forward from:');
    if (containers.length === 1) {
      console.log(
        chalk.cyan(
          `Using container: ${container.name}${container.managed ? ' (devcontainer)' : ''}`,
        ),
      );
    }

    const { alias, targetHost } = await resolveTarget(
      container,
      aliases,
      containers,
      state,
      flags.interactive,
    );

    const portsStr = await input(
      'ports',
      `Ports to forward from '${container.name}' (e.g. 3000, 8080:80):`,
      undefined,
      (v) => {
        try {
          parsePortsList(v);
          return true;
        } catch (e) {
          return (e as Error).message;
        }
      },
    );

    for (const pair of parsePortsList(portsStr)) {
      tunnels.push({
        localPort: pair.localPort,
        containerPort: pair.containerPort,
        targetHost,
        alias,
        containerName: container.name,
        isDevcontainer: container.managed,
      });
    }

    const more = await confirm('more', 'Add ports from another container?', false);
    if (!more) break;
  }

  return tunnels;
}

function printPlan(tunnels: PlannedTunnel[]): void {
  console.log(chalk.cyan('\nPort forwarding plan:'));
  const maxName = Math.max(...tunnels.map((t) => t.containerName.length));
  for (const t of tunnels) {
    const tag = t.isDevcontainer ? chalk.green('(devcontainer)') : chalk.gray('              ');
    console.log(
      `  ${t.containerName.padEnd(maxName)}  ${tag}  ` +
        chalk.yellow(`localhost:${t.localPort}`) +
        ` → ${t.targetHost}:${t.containerPort}  ` +
        chalk.gray(`(via ${t.alias})`),
    );
  }
  console.log('');
}

function runTunnels(tunnels: PlannedTunnel[]): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const children: ChildProcess[] = [];
    let remaining = tunnels.length;
    let firstError: Error | null = null;
    let terminating = false;
    let settled = false;

    const killAll = () => {
      for (const c of children) {
        if (c.exitCode === null && !c.killed) {
          try {
            c.kill('SIGTERM');
          } catch {
            /* already gone */
          }
        }
      }
    };

    const onSigint = () => {
      terminating = true;
      killAll();
    };
    process.on('SIGINT', onSigint);

    const finish = () => {
      if (settled) return;
      settled = true;
      process.removeListener('SIGINT', onSigint);
      if (firstError) {
        reject(firstError);
      } else {
        console.log(chalk.green(`\nAll SSH tunnels closed.\n`));
        resolve();
      }
    };

    for (const t of tunnels) {
      const child = spawn('ssh', ['-N', '-L', `${t.localPort}:${t.targetHost}:${t.containerPort}`, t.alias], {
        stdio: 'inherit',
      });
      children.push(child);

      child.on('error', (err) => {
        if (!firstError) firstError = new Error(`Failed to spawn SSH process: ${err.message}`);
        terminating = true;
        killAll();
      });

      child.on('close', (code) => {
        remaining--;
        if (code !== 0 && code !== null && !terminating && !firstError) {
          firstError = new Error(
            `SSH tunnel ${t.localPort}→${t.targetHost}:${t.containerPort} (${t.alias}) closed with exit code ${code}`,
          );
          // One failing tunnel tears down the rest so the user isn't left with a
          // partial set silently running in the background.
          terminating = true;
          killAll();
        }
        if (remaining === 0) finish();
      });
    }
  });
}

export async function runPortForward(argv: string[]): Promise<void> {
  const flags = parsePortForwardFlags(argv);
  if (flags.help) {
    console.log(portForwardHelp());
    return;
  }

  // No explicit mapping + interactive → multi-container picker.
  if (!flags.portMapping && flags.interactive) {
    const tunnels = await buildTunnelsInteractive(flags);
    printPlan(tunnels);
    const ok = await confirm(
      'start',
      `Open ${tunnels.length} tunnel${tunnels.length === 1 ? '' : 's'} now?`,
      true,
    );
    if (!ok) {
      console.log(chalk.yellow('Aborted. No tunnels were opened.'));
      return;
    }
    console.log(chalk.green('Press Ctrl+C to terminate the port forwarding session.\n'));
    return runTunnels(tunnels);
  }

  // Explicit single mapping (positional or non-interactive).
  if (!flags.portMapping) {
    throw new Error('Port mapping is required in non-interactive mode.');
  }

  const mapping = parsePortMapping(flags.portMapping, flags.service);

  let alias = flags.alias;
  let containerName = alias ?? mapping.targetHost;
  let isDevcontainer = true;
  if (!alias) {
    const container = await pickManagedContainer('Select devcontainer to forward into:', {
      interactive: flags.interactive,
    });
    containerName = container.name;
    const ws = containerWorkspace(container.name);
    const candidate = ws ?? container.name;
    const aliases = getSshAliases();
    if (aliases.includes(candidate)) {
      alias = candidate;
    } else if (aliases.length === 0) {
      if (!flags.interactive) {
        throw new Error(
          `No SSH aliases found in config. Run 'setup-ssh' first or specify --alias.`,
        );
      }
      alias = await input(
        'alias',
        `No SSH alias found for '${container.name}'. Enter alias manually:`,
        candidate,
        (v) => (v.trim() ? true : 'SSH alias cannot be empty.'),
      );
    } else if (aliases.length === 1) {
      alias = aliases[0];
      console.log(chalk.cyan(`Using SSH alias '${alias}'.`));
    } else {
      if (!flags.interactive) {
        throw new Error(
          `SSH alias '${candidate}' not found in ~/.ssh/config. Specify --alias or run interactively.`,
        );
      }
      alias = await select(
        'alias',
        `Select SSH alias for container '${container.name}':`,
        aliases.map((a) => ({ name: a, message: a })),
      );
    }
  }

  const tunnel: PlannedTunnel = {
    localPort: mapping.localPort,
    containerPort: mapping.containerPort,
    targetHost: mapping.targetHost,
    alias,
    containerName,
    isDevcontainer,
  };

  printPlan([tunnel]);
  console.log(chalk.green('Press Ctrl+C to terminate the port forwarding session.\n'));
  return runTunnels([tunnel]);
}
