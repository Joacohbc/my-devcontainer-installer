import * as os from 'os';
import * as path from 'path';

export const SSH_DEFAULTS = {
  user: 'devuser',
  serviceName: 'devcontainer-ssh',
  alias: 'devcontainer',
  keyName: 'id_devcontainer',
  windowsPort: 2222,
  dockerIpFormat: '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',
} as const;

export function defaultKeyPath(): string {
  return path.join(os.homedir(), '.ssh', SSH_DEFAULTS.keyName);
}

export function authorizedKeysInstallScript(): string {
  return 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && sort -u ~/.ssh/authorized_keys -o ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys';
}

export type SshConfigMode = 'local' | 'windows' | 'remote';

export interface SshConfigBlockOptions {
  mode: SshConfigMode;
  alias: string;
  user: string;
  key: string;
  hostname?: string;
  port?: number | string;
  remote?: string;
  container?: string;
}

export function buildSshConfigBlock(opts: SshConfigBlockOptions): string {
  const { mode, alias, user, key } = opts;
  const lines = [`Host ${alias}`];
  switch (mode) {
    case 'local':
      if (!opts.hostname) throw new Error('hostname required for local mode');
      lines.push(`    HostName ${opts.hostname}`);
      lines.push(`    User ${user}`);
      lines.push(`    IdentityFile ${key}`);
      break;
    case 'windows':
      lines.push(`    HostName ${opts.hostname ?? 'localhost'}`);
      lines.push(`    Port ${opts.port ?? SSH_DEFAULTS.windowsPort}`);
      lines.push(`    User ${user}`);
      lines.push(`    IdentityFile ${key}`);
      break;
    case 'remote': {
      if (!opts.remote) throw new Error('remote required for remote mode');
      if (!opts.container) throw new Error('container required for remote mode');
      const ipExpr = `$(docker inspect -f '${SSH_DEFAULTS.dockerIpFormat}' ${opts.container})`;
      lines.push(`    User ${user}`);
      lines.push(`    IdentityFile ${key}`);
      lines.push(`    ProxyCommand ssh ${opts.remote} "nc -q0 ${ipExpr} 22"`);
      break;
    }
  }
  return lines.join('\n');
}
