import type { DockerfileModule } from '../../types.js';

export const pnpmModule: DockerfileModule = {
  id: 'pnpm',
  label: 'pnpm (for devuser)',
  category: 'runtime',
  requires: ['nodejs'],
  render() {
    return `##
## PNPM (devuser)
##
RUN su - devuser -c 'wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh -'
`;
  },
};
