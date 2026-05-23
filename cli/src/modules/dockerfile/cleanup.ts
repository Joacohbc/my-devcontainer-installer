import type { DockerfileModule } from '@/types.js';

export const cleanupModule: DockerfileModule = {
  id: 'cleanup',
  label: 'Cleanup + entrypoint + EXPOSE 22',
  category: 'cleanup',
  always: true,
  copyFiles: ['entrypoint.sh'],
  postScriptFiles: ['login-github-cli.sh'],
  render() {
    return `##
## CLEANUP & ENTRYPOINT
##
RUN apt-get autoremove -y && \\
    apt-get autoclean && \\
    rm -rf /var/lib/apt/lists/* && \\
    rm -rf /tmp/* && \\
    rm -rf /var/tmp/*

RUN mkdir -p /var/run/sshd && \\
    chmod 755 /var/run/sshd

EXPOSE 22

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
`;
  },
};
