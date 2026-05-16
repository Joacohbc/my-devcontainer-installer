import chalk from 'chalk';

const USER = 'devuser';
const KEY = '~/.ssh/id_devcontainer';
const HOST_ALIAS = 'devcontainer';

function linuxBlock(): string {
  const prompt = chalk.gray('   $ ');
  const ip = `IP_SSH=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${HOST_ALIAS}-ssh)`;
  return [
    chalk.bold('4) Install key + register host (Linux / Mac, direct container IP):'),
    prompt + ip,
    prompt + `ssh-copy-id -i ${KEY}.pub ${USER}@$IP_SSH`,
    prompt + `cat <<EOF >> ~/.ssh/config
Host ${HOST_ALIAS}
    HostName $IP_SSH
    IdentityFile ${KEY}
    User ${USER}
EOF`,
  ].join('\n');
}

function windowsBlock(): string {
  const prompt = chalk.gray('   $ ');
  return [
    chalk.bold('4) Install key + register host (Windows / Git Bash, port 2222):'),
    prompt + `ssh-copy-id -p 2222 -i ${KEY}.pub ${USER}@localhost`,
    prompt + `cat <<EOF >> ~/.ssh/config
Host ${HOST_ALIAS}
    HostName localhost
    Port 2222
    User ${USER}
    IdentityFile ${KEY}
EOF`,
  ].join('\n');
}

export function printSshInstructions(): void {
  const isWindows = process.platform === 'win32';
  const bar = chalk.gray('─'.repeat(64));
  const prompt = chalk.gray('   $ ');
  const startCmd = isWindows
    ? 'docker compose -f docker-compose.yml -f docker-compose.windows.yml up -d'
    : 'docker compose up -d';

  const out = [
    chalk.cyan.bold('🔑 Next steps — SSH access'),
    bar,
    '',
    chalk.bold('1) Start the stack (if not running):'),
    prompt + startCmd,
    '',
    chalk.bold('2) Get temporary password (one-time, to install your key):'),
    prompt + `docker compose logs ${HOST_ALIAS}-ssh | grep "${USER} password" | tail -n 1`,
    '',
    chalk.bold(`3) Generate SSH key (skip if you already have ${KEY}):`),
    prompt + `ssh-keygen -t ed25519 -f ${KEY} -N "" -q`,
    '',
    isWindows ? windowsBlock() : linuxBlock(),
    '',
    chalk.bold('5) Connect:'),
    prompt + `ssh ${HOST_ALIAS}`,
    '',
    chalk.gray('For remote-server access (ProxyCommand) see README.md → "Acceso y Uso".'),
    bar,
    '',
  ].join('\n');

  console.log(out);
}
