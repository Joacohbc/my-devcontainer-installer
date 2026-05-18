import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { spawnSync, type SpawnSyncOptions } from 'child_process';
import chalk from 'chalk';
import { confirm, PromptCancelledError } from './prompts.js';

export interface SetupSshFlags {
  remote: string;
  alias: string;
  key: string;
  port: string;
  mode: '' | 'local' | 'windows' | 'remote';
  assumeYes: boolean;
  container: string;
  user: string;
  help: boolean;
}

const DEFAULTS: SetupSshFlags = {
  remote: '',
  alias: 'devcontainer',
  key: path.join(os.homedir(), '.ssh', 'id_devcontainer'),
  port: '2222',
  mode: '',
  assumeYes: false,
  container: 'devcontainer-ssh',
  user: 'devuser',
  help: false,
};

export function parseSetupSshFlags(argv: string[]): SetupSshFlags {
  const f: SetupSshFlags = { ...DEFAULTS };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => argv[++i];
    switch (a) {
      case '-h':
      case '--help':
        f.help = true;
        break;
      case '--remote':
        f.remote = next();
        f.mode = 'remote';
        break;
      case '--alias':
        f.alias = next();
        break;
      case '--key':
        f.key = next();
        break;
      case '--port':
        f.port = next();
        break;
      case '--mode': {
        const v = next();
        if (v !== 'local' && v !== 'windows' && v !== 'remote') {
          throw new Error(`Invalid --mode: ${v}`);
        }
        f.mode = v;
        break;
      }
      case '--container':
        f.container = next();
        break;
      case '--user':
        f.user = next();
        break;
      case '-y':
      case '--yes':
        f.assumeYes = true;
        break;
      default:
        throw new Error(`Unknown flag: ${a}`);
    }
  }
  return f;
}

export function setupSshHelp(): string {
  return `cli setup-ssh — automate SSH key + config for devcontainer-ssh

Usage:
  cli setup-ssh [flags]

Flags:
  --remote USER@HOST   Configure remote-server access (ProxyCommand mode).
  --alias NAME         SSH alias to register (default: ${DEFAULTS.alias}).
  --key PATH           Private key path (default: ${DEFAULTS.key}).
  --port PORT          Port for Windows mode (default: ${DEFAULTS.port}).
  --mode MODE          Force mode: local | windows | remote.
  --container NAME     Container name (default: ${DEFAULTS.container}).
  --user NAME          SSH user inside container (default: ${DEFAULTS.user}).
  -y, --yes            Assume "yes" to all prompts.
  -h, --help           Show this help.
`;
}

const log = (s: string) => console.log(`${chalk.blue.bold('==>')} ${s}`);
const ok = (s: string) => console.log(`${chalk.green.bold('✓')}   ${s}`);
const warn = (s: string) => console.log(`${chalk.yellow.bold('!')}   ${s}`);
const dim = (s: string) => console.log(chalk.gray(s));

function run(cmd: string, args: string[], opts: SpawnSyncOptions = {}) {
  const r = spawnSync(cmd, args, { encoding: 'utf8', ...opts });
  return { status: r.status, stdout: String(r.stdout ?? ''), stderr: String(r.stderr ?? '') };
}

function runInherit(cmd: string, args: string[]) {
  return spawnSync(cmd, args, { stdio: 'inherit' });
}

function which(bin: string): boolean {
  const r = run('sh', ['-c', `command -v ${bin}`]);
  return r.status === 0 && r.stdout.trim().length > 0;
}

type Mode = 'local' | 'windows' | 'remote';

function detectMode(f: SetupSshFlags): Mode {
  if (f.mode) return f.mode;
  if (process.platform === 'win32') return 'windows';
  return 'local';
}

function checkPrereqs(mode: Mode) {
  for (const t of ['ssh', 'ssh-keygen']) {
    if (!which(t)) throw new Error(`Missing tool: ${t}`);
  }
  if (mode === 'local' || mode === 'windows') {
    if (!which('docker')) throw new Error('Missing tool: docker');
  }
}

function stackRunning(container: string): boolean {
  const r = run('docker', ['ps', '--format', '{{.Names}}']);
  if (r.status !== 0) return false;
  return r.stdout.split('\n').some((l) => l.trim() === container);
}

async function ensureStack(f: SetupSshFlags, mode: Mode): Promise<void> {
  if (mode === 'remote') return;
  if (stackRunning(f.container)) {
    ok(`Container '${f.container}' is running.`);
    return;
  }
  warn(`Container '${f.container}' not running.`);
  const proceed = f.assumeYes
    ? true
    : await confirm('startStack', 'Start it now with docker compose up -d?', true);
  if (!proceed) throw new Error('Aborting — stack must be running.');

  const composeArgs =
    mode === 'windows'
      ? ['compose', '-f', 'docker-compose.yml', '-f', 'docker-compose.windows.yml', 'up', '-d']
      : ['compose', 'up', '-d'];
  const r = runInherit('docker', composeArgs);
  if (r.status !== 0) throw new Error('docker compose up failed.');

  let tries = 20;
  while (tries > 0 && !stackRunning(f.container)) {
    await new Promise((res) => setTimeout(res, 1000));
    tries--;
  }
  if (!stackRunning(f.container)) throw new Error('Container failed to start.');
}

function fetchPassword(f: SetupSshFlags, mode: Mode): void {
  if (mode === 'remote') return;
  log('Fetching temporary password from logs...');
  const r = run('docker', ['compose', 'logs', f.container]);
  if (r.status !== 0) {
    warn('Could not read compose logs.');
    return;
  }
  const lines = r.stdout
    .split('\n')
    .filter((l) => l.includes(`${f.user} password`));
  const last = lines[lines.length - 1];
  if (!last) {
    warn('Could not read password from logs (maybe key already installed).');
  } else {
    dim(`   ${last}`);
  }
}

function genKey(keyPath: string): void {
  if (fs.existsSync(keyPath) && fs.existsSync(`${keyPath}.pub`)) {
    ok(`Key already exists: ${keyPath}`);
    return;
  }
  log(`Generating ed25519 key at ${keyPath}`);
  fs.mkdirSync(path.dirname(keyPath), { recursive: true });
  const r = runInherit('ssh-keygen', ['-t', 'ed25519', '-f', keyPath, '-N', '', '-q']);
  if (r.status !== 0) throw new Error('ssh-keygen failed.');
  ok('Key generated.');
}

function containerIp(container: string): string {
  const r = run('docker', [
    'inspect',
    '-f',
    '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',
    container,
  ]);
  if (r.status !== 0) throw new Error('Could not resolve container IP.');
  return r.stdout.trim();
}

interface InstallResult {
  hostname: string;
  port: string;
}

function installKeyLocal(f: SetupSshFlags): InstallResult {
  const ip = containerIp(f.container);
  if (!ip) throw new Error('Could not resolve container IP.');
  log(`Installing public key into ${f.container} (${ip})...`);
  const r = runInherit('ssh-copy-id', [
    '-o',
    'StrictHostKeyChecking=accept-new',
    '-i',
    `${f.key}.pub`,
    `${f.user}@${ip}`,
  ]);
  if (r.status !== 0) throw new Error('ssh-copy-id failed.');
  return { hostname: ip, port: '' };
}

function installKeyWindows(f: SetupSshFlags): InstallResult {
  log(`Installing public key via localhost:${f.port}...`);
  if (which('ssh-copy-id')) {
    const r = runInherit('ssh-copy-id', [
      '-o',
      'StrictHostKeyChecking=accept-new',
      '-p',
      f.port,
      '-i',
      `${f.key}.pub`,
      `${f.user}@localhost`,
    ]);
    if (r.status !== 0) throw new Error('ssh-copy-id failed.');
  } else {
    warn('ssh-copy-id missing — using manual fallback.');
    const pub = fs.readFileSync(`${f.key}.pub`);
    const r = spawnSync(
      'ssh',
      [
        '-o',
        'StrictHostKeyChecking=accept-new',
        '-p',
        f.port,
        `${f.user}@localhost`,
        'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys',
      ],
      { input: pub, stdio: ['pipe', 'inherit', 'inherit'] },
    );
    if (r.status !== 0) throw new Error('ssh key install failed.');
  }
  return { hostname: 'localhost', port: f.port };
}

function installKeyRemote(f: SetupSshFlags): InstallResult {
  if (!f.remote) throw new Error('--remote USER@HOST required for remote mode.');
  log(`Installing public key into ${f.container} via ${f.remote}...`);
  const pub = fs.readFileSync(`${f.key}.pub`);
  const remoteCmd = `docker exec -i -u ${f.user} ${f.container} sh -c 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys'`;
  const r = spawnSync('ssh', [f.remote, remoteCmd], {
    input: pub,
    stdio: ['pipe', 'inherit', 'inherit'],
  });
  if (r.status !== 0) throw new Error('Remote key install failed.');
  return { hostname: '', port: '' };
}

export function buildConfigBlock(
  mode: Mode,
  f: SetupSshFlags,
  inst: InstallResult,
): string {
  switch (mode) {
    case 'local':
      return [
        `Host ${f.alias}`,
        `    HostName ${inst.hostname}`,
        `    User ${f.user}`,
        `    IdentityFile ${f.key}`,
      ].join('\n');
    case 'windows':
      return [
        `Host ${f.alias}`,
        `    HostName ${inst.hostname}`,
        `    Port ${inst.port}`,
        `    User ${f.user}`,
        `    IdentityFile ${f.key}`,
      ].join('\n');
    case 'remote': {
      const ipExpr = `$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${f.container})`;
      return [
        `Host ${f.alias}`,
        `    User ${f.user}`,
        `    IdentityFile ${f.key}`,
        `    ProxyCommand ssh ${f.remote} "nc -q0 ${ipExpr} 22"`,
      ].join('\n');
    }
  }
}

function aliasOfHostLine(line: string): string[] {
  const m = line.match(/^\s*Host\s+(.+)$/);
  if (!m) return [];
  return m[1].split(/\s+/).filter(Boolean);
}

export function hasAliasBlock(content: string, alias: string): boolean {
  for (const line of content.split('\n')) {
    if (aliasOfHostLine(line).includes(alias)) return true;
  }
  return false;
}

export function stripAliasBlock(content: string, alias: string): string {
  const lines = content.split('\n');
  const out: string[] = [];
  let skip = false;
  for (const line of lines) {
    const hosts = aliasOfHostLine(line);
    const isHostLine = hosts.length > 0;
    if (skip) {
      if (isHostLine) {
        if (hosts.includes(alias)) continue;
        skip = false;
        out.push(line);
      }
      continue;
    }
    if (isHostLine && hosts.includes(alias)) {
      skip = true;
      continue;
    }
    out.push(line);
  }
  return out.join('\n');
}

function extractAliasBlock(content: string, alias: string): string {
  const lines = content.split('\n');
  const out: string[] = [];
  let printing = false;
  for (const line of lines) {
    const hosts = aliasOfHostLine(line);
    const isHostLine = hosts.length > 0;
    if (printing) {
      if (isHostLine) {
        if (!hosts.includes(alias)) break;
        out.push(line);
        continue;
      }
      out.push(line);
      continue;
    }
    if (isHostLine && hosts.includes(alias)) {
      printing = true;
      out.push(line);
    }
  }
  return out.join('\n');
}

async function updateSshConfig(
  f: SetupSshFlags,
  mode: Mode,
  inst: InstallResult,
): Promise<void> {
  const sshDir = path.join(os.homedir(), '.ssh');
  const configPath = path.join(sshDir, 'config');
  fs.mkdirSync(sshDir, { recursive: true });
  if (!fs.existsSync(configPath)) fs.writeFileSync(configPath, '');
  fs.chmodSync(configPath, 0o600);

  const newBlock = buildConfigBlock(mode, f, inst);
  const current = fs.readFileSync(configPath, 'utf8');

  if (hasAliasBlock(current, f.alias)) {
    warn(`Host '${f.alias}' already defined in ${configPath}`);
    dim('---- existing ----');
    console.log(extractAliasBlock(current, f.alias));
    dim('---- proposed ----');
    console.log(newBlock);
    const replace = f.assumeYes
      ? true
      : await confirm('replaceAlias', `Replace existing block for Host '${f.alias}'?`, false);
    if (!replace) {
      warn('Skipping ssh config update.');
      return;
    }
    fs.copyFileSync(configPath, `${configPath}.bak`);
    ok(`Backup saved: ${configPath}.bak`);
    let stripped = stripAliasBlock(current, f.alias);
    if (stripped.length > 0 && !stripped.endsWith('\n')) stripped += '\n';
    fs.writeFileSync(configPath, `${stripped}${newBlock}\n`);
    fs.chmodSync(configPath, 0o600);
    ok(`Replaced Host '${f.alias}' in ${configPath}`);
  } else {
    let body = current;
    if (body.length > 0 && !body.endsWith('\n')) body += '\n';
    fs.writeFileSync(configPath, `${body}${newBlock}\n`);
    fs.chmodSync(configPath, 0o600);
    ok(`Appended Host '${f.alias}' to ${configPath}`);
  }
}

function testConnection(alias: string): void {
  log(`Testing ssh ${alias} ...`);
  const r = run('ssh', [
    '-o',
    'BatchMode=yes',
    '-o',
    'StrictHostKeyChecking=accept-new',
    alias,
    'echo OK',
  ]);
  if (r.status === 0 && r.stdout.split('\n').some((l) => l.trim() === 'OK')) {
    ok(`SSH alias '${alias}' works.`);
  } else {
    warn(`SSH test inconclusive. Try manually:  ssh ${alias}`);
  }
}

export async function runSetupSsh(argv: string[]): Promise<void> {
  const f = parseSetupSshFlags(argv);
  if (f.help) {
    console.log(setupSshHelp());
    return;
  }
  const mode = detectMode(f);
  log(`Mode: ${mode}   Alias: ${f.alias}   Key: ${f.key}`);
  checkPrereqs(mode);
  await ensureStack(f, mode);
  fetchPassword(f, mode);
  genKey(f.key);

  const installers: Record<Mode, (f: SetupSshFlags) => InstallResult> = {
    local: installKeyLocal,
    windows: installKeyWindows,
    remote: installKeyRemote,
  };
  const inst = installers[mode](f);

  await updateSshConfig(f, mode, inst);
  testConnection(f.alias);
  ok(`Done. Connect with:  ssh ${f.alias}`);
}

export async function runSetupSshMain(argv: string[]): Promise<void> {
  try {
    await runSetupSsh(argv);
  } catch (e) {
    if (e instanceof PromptCancelledError) {
      console.log(chalk.yellow('\nCancelled.'));
      process.exit(130);
    }
    throw e;
  }
}
