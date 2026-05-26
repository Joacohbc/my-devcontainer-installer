import type { DockerfileModule } from '@/core/types.js';
import { emitShellInit } from '@/modules/dockerfile/shell-init.js';

export const nodejsModule: DockerfileModule = {
  id: 'nodejs',
  label: 'Node.js (nvm or fnm, for devuser)',
  category: 'runtime',
  requires: ['github-cli'],
  options: [
    {
      id: 'manager',
      label: 'Node.js Version manager',
      type: 'select',
      choices: [
        { value: 'nvm', label: 'nvm (Node Version Manager)' },
        { value: 'fnm', label: 'fnm (Fast Node Manager, Rust)' },
      ],
      default: 'nvm',
    },
    {
      id: 'version',
      label: 'Node version',
      type: 'select',
      choices: [
        { value: 'lts', label: 'LTS (auto)' },
        { value: '22', label: 'Node 22 (Maintenance LTS)' },
        { value: '24', label: 'Node 24 (Active LTS)' },
      ],
      default: 'lts',
    },
  ],
  render(opts) {
    const manager = (opts.manager as string) || 'nvm';
    const version = (opts.version as string) || 'lts';
    return manager === 'fnm' ? renderFnm(version) : renderNvm(version);
  },
};

function renderNvm(version: string): string {
  const useLine =
    version === 'lts' ? 'nvm use --lts > /dev/null' : `nvm use ${version} > /dev/null`;
  const installNode =
    version === 'lts' ? 'nvm install --lts' : `nvm install ${version}`;
  const initLines = [
    'export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"',
    '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"',
    useLine,
  ];
  return `##
## NVM + NODE (devuser)
##
RUN NVM_VERSION=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .tag_name) && \\
    su - devuser -c "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/\${NVM_VERSION}/install.sh | bash"
RUN su - devuser -c 'export NVM_DIR="$HOME/.nvm" && . "$NVM_DIR/nvm.sh" && ${installNode}'
${emitShellInit('.nodejs_init.sh', initLines)}
`;
}

function renderFnm(version: string): string {
  const useLine =
    version === 'lts' ? 'fnm use lts-latest > /dev/null' : `fnm use ${version} > /dev/null`;
  const installNode =
    version === 'lts' ? 'fnm install --lts' : `fnm install ${version}`;
  const initLines = [
    'export PATH="$HOME/.fnm:$PATH"',
    'eval "$(fnm env --use-on-cd)"',
    useLine,
  ];
  return `##
## FNM + NODE (devuser)
##
RUN su - devuser -c 'curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$HOME/.fnm" --skip-shell'
RUN su - devuser -c 'export PATH="$HOME/.fnm:$PATH" && eval "$(fnm env)" && ${installNode}'
${emitShellInit('.nodejs_init.sh', initLines)}
`;
}
