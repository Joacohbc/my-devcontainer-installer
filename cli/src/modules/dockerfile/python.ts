import type { DockerfileModule } from '@/core/types.js';
import { emitShellInit } from '@/modules/dockerfile/shell-init.js';

export const pythonModule: DockerfileModule = {
  id: 'python',
  label: 'Python (python3 + pip, optional uv)',
  category: 'lang',
  options: [
    {
      id: 'uv',
      label: 'Install uv (Astral, for devuser)',
      type: 'confirm',
      default: true,
    },
  ],
  render(opts) {
    const uv = opts.uv !== false;
    const uvBlock = uv
      ? `
# Install uv (Astral Python installer/manager) for devuser
RUN su - devuser -c 'curl -LsSf https://astral.sh/uv/install.sh | sh'
${emitShellInit('.python_init.sh', ['export PATH="$HOME/.local/bin:$PATH"'])}
`
      : '';
    return `##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip
${uvBlock}`;
  },
};
