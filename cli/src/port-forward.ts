import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { spawn } from 'child_process';
import chalk from 'chalk';
import { input, select, PromptCancelledError } from './prompts.js';

export interface PortForwardFlags {
  portMapping: string | null;
  alias: string | null;
  service: string | null;
}

export function parsePortForwardFlags(argv: string[]): PortForwardFlags {
  const flags: PortForwardFlags = {
    portMapping: null,
    alias: null,
    service: null,
  };

  let portMappingSeen = false;

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];

    if (arg === '--alias') {
      if (flags.alias !== null) {
        throw new Error('Duplicate flag: --alias');
      }
      if (i + 1 >= argv.length) {
        throw new Error('Missing value for --alias');
      }
      flags.alias = argv[++i];
    } else if (arg === '--service') {
      if (flags.service !== null) {
        throw new Error('Duplicate flag: --service');
      }
      if (i + 1 >= argv.length) {
        throw new Error('Missing value for --service');
      }
      flags.service = argv[++i];
    } else if (arg.startsWith('--')) {
      throw new Error(`Unknown flag: ${arg}`);
    } else {
      if (portMappingSeen) {
        throw new Error(`Too many positional arguments: ${arg}`);
      }
      flags.portMapping = arg;
      portMappingSeen = true;
    }
  }

  return flags;
}

export function getAvailableSshHosts(sshConfigPath: string): string[] {
  if (!fs.existsSync(sshConfigPath)) {
    return [];
  }
  const content = fs.readFileSync(sshConfigPath, 'utf8');
  // Regex to match "Host <name>" ignoring "Host *"
  const hostRegex = /^[ \t]*Host[ \t]+([^#\n]+)/gmi;
  const hosts = new Set<string>();
  let match;
  while ((match = hostRegex.exec(content)) !== null) {
    const hostNames = match[1].split(/[ \t]+/).filter(Boolean);
    for (const h of hostNames) {
      if (h !== '*') {
        hosts.add(h);
      }
    }
  }
  return Array.from(hosts);
}

export async function runPortForward(argv: string[], sshConfigPathOverride?: string): Promise<void> {
  let flags: PortForwardFlags;
  try {
    flags = parsePortForwardFlags(argv);
  } catch (e: any) {
    throw new Error(e.message);
  }

  let localPort: string;
  let remoteHost: string;
  let remotePort: string;

  // 1. Resolve Port Mapping
  if (flags.portMapping) {
    const parts = flags.portMapping.split(':');
    if (parts.length === 1) {
      localPort = parts[0];
      remoteHost = flags.service || 'localhost';
      remotePort = parts[0];
    } else if (parts.length === 2) {
      localPort = parts[0];
      remoteHost = flags.service || 'localhost';
      remotePort = parts[1];
    } else if (parts.length === 3) {
      localPort = parts[0];
      remoteHost = parts[1];
      remotePort = parts[2];
      if (flags.service && flags.service !== remoteHost) {
        throw new Error(`Conflicting service definitions: ${flags.service} vs ${remoteHost}`);
      }
    } else {
      throw new Error(`Invalid port mapping format: ${flags.portMapping}`);
    }
  } else {
    // Interactive prompt for ports
    const inputMapping = await input(
      'portMapping',
      'Enter port mapping (e.g., 3000, 8080:80, or 5432:postgres:5432):'
    );
    const parts = inputMapping.split(':');
    if (parts.length === 1) {
      localPort = parts[0];
      remoteHost = flags.service || 'localhost';
      remotePort = parts[0];
    } else if (parts.length === 2) {
      localPort = parts[0];
      remoteHost = flags.service || 'localhost';
      remotePort = parts[1];
    } else if (parts.length === 3) {
      localPort = parts[0];
      remoteHost = parts[1];
      remotePort = parts[2];
      if (flags.service && flags.service !== remoteHost) {
        throw new Error(`Conflicting service definitions: ${flags.service} vs ${remoteHost}`);
      }
    } else {
      throw new Error(`Invalid port mapping format: ${inputMapping}`);
    }
  }

  // 2. Resolve SSH Alias
  let alias = flags.alias;
  if (!alias) {
    const sshConfigPath = sshConfigPathOverride || path.join(os.homedir(), '.ssh', 'config');
    const availableHosts = getAvailableSshHosts(sshConfigPath);

    if (availableHosts.length === 0) {
      // Fallback if no config or no hosts
      alias = await input(
        'alias',
        'No SSH hosts found. Enter SSH alias to use (e.g., devcontainer):',
        'devcontainer'
      );
    } else if (availableHosts.length === 1) {
      // Auto-select if only one host
      alias = availableHosts[0];
      console.log(chalk.gray(`Auto-selected only available SSH host: ${alias}`));
    } else {
      // Prompt user to select from available hosts
      alias = await select(
        'alias',
        'Select SSH host to tunnel through:',
        availableHosts.map((h) => ({ name: h, message: h }))
      );
    }
  }

  console.log(chalk.green(`\n🚀 Forwarding local port ${localPort} -> ${remoteHost}:${remotePort} via ${alias}`));
  console.log(chalk.gray(`Press Ctrl+C to stop forwarding.\n`));

  // 3. Spawn SSH Tunnel
  return new Promise((resolve, reject) => {
    const sshArgs = [
      '-N', // Do not execute a remote command
      '-L', `${localPort}:${remoteHost}:${remotePort}`,
      alias!
    ];

    const child = spawn('ssh', sshArgs, { stdio: 'inherit' });

    child.on('error', (err) => {
      reject(new Error(`Failed to spawn ssh: ${err.message}`));
    });

    child.on('close', (code) => {
      if (code !== 0 && code !== null) {
        reject(new Error(`SSH process exited with code ${code}`));
      } else {
        resolve();
      }
    });

    // Handle process termination correctly
    const handleExit = () => {
      child.kill('SIGINT');
    };

    process.on('SIGINT', handleExit);
    process.on('SIGTERM', handleExit);
  });
}
