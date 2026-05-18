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
RUN apt-get update && apt-get install -y --no-install-recommends libatomic1 && rm -rf /var/lib/apt/lists/*
RUN su - devuser -c 'wget -qO- https://get.pnpm.io/install.sh | ENV="$HOME/.profile" SHELL="$(which zsh)" zsh -'
`;
  },
};
