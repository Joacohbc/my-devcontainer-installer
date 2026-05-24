import type { DockerfileModule } from '@/types.js';

export const tmuxModule: DockerfileModule = {
  id: 'tmux',
  label: 'Tmux',
  category: 'infra',
  render() {
    return `##
## TMUX
##
RUN apt-get update && apt-get install -y tmux
`;
  },
};
