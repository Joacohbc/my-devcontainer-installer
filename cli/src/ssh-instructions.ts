import chalk from 'chalk';
import { SSH_DEFAULTS, buildSshConfigBlock } from './ssh-defaults.js';

const KEY = `~/.ssh/${SSH_DEFAULTS.keyName}`;

function linuxBlock(): string {
  const prompt = chalk.gray('   $ ');
  const ipVar = 'IP_SSH';
  const ipCmd = `${ipVar}=$(docker inspect -f '${SSH_DEFAULTS.dockerIpFormat}' ${SSH_DEFAULTS.serviceName})`;
  const block = buildSshConfigBlock({
    mode: 'local',
    alias: SSH_DEFAULTS.alias,
    user: SSH_DEFAULTS.user,
    key: KEY,
    hostname: `$${ipVar}`,
  });
  return [
    chalk.bold('4) Install key + register host (Linux / Mac, direct container IP):'),
    prompt + ipCmd,
    prompt + `ssh-copy-id -i ${KEY}.pub ${SSH_DEFAULTS.user}@$${ipVar}`,
    prompt + `cat <<EOF >> ~/.ssh/config\n\n${block}\nEOF`,
  ].join('\n');
}

function windowsBlock(): string {
  const prompt = chalk.gray('   $ ');
  const block = buildSshConfigBlock({
    mode: 'windows',
    alias: SSH_DEFAULTS.alias,
    user: SSH_DEFAULTS.user,
    key: KEY,
    hostname: 'localhost',
    port: SSH_DEFAULTS.windowsPort,
  });
  return [
    chalk.bold(`4) Install key + register host (Windows / Git Bash, port ${SSH_DEFAULTS.windowsPort}):`),
    prompt + `ssh-copy-id -p ${SSH_DEFAULTS.windowsPort} -i ${KEY}.pub ${SSH_DEFAULTS.user}@localhost`,
    prompt + `cat <<EOF >> ~/.ssh/config\n\n${block}\nEOF`,
  ].join('\n');
}

export function printSshInstructions(workspace: string): void {
  const isWindows = process.platform === 'win32';
  const bar = chalk.gray('─'.repeat(64));
  const prompt = chalk.gray('   $ ');
  const composeRel = `.dc_${workspace}/build/docker-compose.yml`;
  const startCmd = `docker compose -f ${composeRel} up -d`;

  const out = [
    chalk.cyan.bold('🔑 Next steps — SSH access'),
    bar,
    '',
    chalk.bold('1) Start the stack (if not running):'),
    prompt + startCmd,
    '',
    chalk.bold('2) Get temporary password (one-time, to install your key):'),
    prompt + `docker compose logs ${SSH_DEFAULTS.serviceName} | grep "${SSH_DEFAULTS.user} password" | tail -n 1`,
    '',
    chalk.bold(`3) Generate SSH key (skip if you already have ${KEY}):`),
    prompt + `ssh-keygen -t ed25519 -f ${KEY} -N "" -q`,
    '',
    isWindows ? windowsBlock() : linuxBlock(),
    '',
    chalk.bold('5) Connect:'),
    prompt + `ssh ${SSH_DEFAULTS.alias}`,
    '',
    chalk.gray('For remote-server access (ProxyCommand) see README.md → "Acceso y Uso".'),
    bar,
    '',
  ].join('\n');

  console.log(out);
}
