import type { DockerfileModule } from '@/core/types.js';

export const goModule: DockerfileModule = {
  id: 'go',
  label: 'Go / Golang (Latest version)',
  category: 'lang',
  copyFiles: ['golang_utils.sh'],
  // golang_utils.sh is also shipped so update_golang.sh can source it at runtime.
  postScriptFiles: ['update_golang.sh', 'golang_utils.sh'],
  render() {
    return `##
## GO
##
COPY golang_utils.sh /tmp/golang_utils.sh
RUN bash -c "source /tmp/golang_utils.sh && install_golang" && rm /tmp/golang_utils.sh
`;
  },
};
