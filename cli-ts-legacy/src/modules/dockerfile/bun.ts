import type { DockerfileModule } from '@/core/types.js';
import { emitShellInit } from '@/modules/dockerfile/shell-init.js';

export const bunModule: DockerfileModule = {
  id: 'bun',
  label: 'Bun',
  category: 'runtime',
  render() {
    return `##
## BUN
##
RUN su - devuser -c "curl -fsSL https://bun.sh/install | bash"
${emitShellInit('.bun_init.sh', [
  'export BUN_INSTALL="$HOME/.bun"',
  'export PATH="$BUN_INSTALL/bin:$PATH"',
])}
`;
  },
};
