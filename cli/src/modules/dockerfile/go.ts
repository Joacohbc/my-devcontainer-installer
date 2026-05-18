import type { DockerfileModule } from '../../types.js';

export const goModule: DockerfileModule = {
  id: 'go',
  label: 'Go / Golang (Latest version)',
  category: 'lang',
  copyFiles: ['golang_utils.sh', 'update_golang.sh'],
  render() {
    return `##
## GO
##
COPY golang_utils.sh /tmp/golang_utils.sh
RUN bash -c "source /tmp/golang_utils.sh && install_golang" && rm /tmp/golang_utils.sh
`;
  },
};
