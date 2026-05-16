import type { DockerfileModule } from '../../types.js';

export const bunModule: DockerfileModule = {
  id: 'bun',
  label: 'Bun',
  category: 'runtime',
  render() {
    return `##
## BUN
##
RUN su - devuser -c "curl -fsSL https://bun.sh/install | bash" && \\
    su - devuser -c 'echo "export BUN_INSTALL=\\"\\$HOME/.bun\\"" >> /home/devuser/.profile' && \\
    su - devuser -c 'echo "export PATH=\\"\\$BUN_INSTALL/bin:\\$PATH\\"" >> /home/devuser/.profile'
`;
  },
};
