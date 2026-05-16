import type { ComposeService } from '../../types.js';

export const devcontainerService: ComposeService = {
  id: 'devcontainer',
  label: 'devcontainer-ssh (main)',
  always: true,
  render({ imageName, enabledServiceIds }) {
    const depends = enabledServiceIds.filter((s) =>
      ['mongo', 'redis', 'postgres'].includes(s),
    );
    const svc: Record<string, unknown> = {
      image: imageName,
      build: '.',
      container_name: 'devcontainer-ssh',
      command: 'sleep infinity',
      restart: 'unless-stopped',
      volumes: [
        '.:/workspace',
        '/var/run/docker.sock:/var/run/docker.sock',
        'devcontainer_etc:/etc',
        'devcontainer_root:/root',
        'devcontainer_home:/home',
      ],
      networks: ['local-network'],
    };
    if (depends.length > 0) svc.depends_on = depends;
    return svc;
  },
};
