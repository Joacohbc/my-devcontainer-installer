import type { ComposeService } from '../../types.js';

export const DOCKER_SOCKET_PROXY_HOST = 'docker-socket-proxy';
export const DOCKER_SOCKET_PROXY_PORT = 2375;

export const dockerSocketProxyService: ComposeService = {
  id: DOCKER_SOCKET_PROXY_HOST,
  label: 'Docker socket proxy (Tecnativa, hardens DooD)',
  options: [
    {
      id: 'allowPost',
      label: 'Allow POST endpoints (container create/start/exec)',
      type: 'confirm',
      default: true,
    },
    {
      id: 'allowExec',
      label: 'Allow EXEC endpoints (docker exec)',
      type: 'confirm',
      default: false,
    },
    {
      id: 'allowVolumes',
      label: 'Allow VOLUMES endpoints (create/remove volumes)',
      type: 'confirm',
      default: false,
    },
  ],
  render({ options }) {
    const allowPost = options.allowPost !== false;
    const allowExec = options.allowExec === true;
    const allowVolumes = options.allowVolumes === true;
    const env: string[] = [
      'CONTAINERS=1',
      'IMAGES=1',
      'NETWORKS=1',
      'INFO=1',
      'PING=1',
      'VERSION=1',
      'EVENTS=1',
      'AUTH=0',
      'SECRETS=0',
      'SERVICES=0',
      'TASKS=0',
      'SWARM=0',
      'NODES=0',
      'PLUGINS=0',
      'SYSTEM=0',
      'CONFIGS=0',
      'DISTRIBUTION=0',
      'SESSION=0',
      'BUILD=0',
      'COMMIT=0',
      `EXEC=${allowExec ? 1 : 0}`,
      `VOLUMES=${allowVolumes ? 1 : 0}`,
      `POST=${allowPost ? 1 : 0}`,
    ];
    return {
      image: 'tecnativa/docker-socket-proxy:latest',
      container_name: DOCKER_SOCKET_PROXY_HOST,
      restart: 'unless-stopped',
      read_only: true,
      tmpfs: ['/run'],
      cap_drop: ['ALL'],
      cap_add: ['CHOWN', 'SETGID', 'SETUID'],
      security_opt: ['no-new-privileges:true'],
      environment: env,
      volumes: ['/var/run/docker.sock:/var/run/docker.sock:ro'],
      networks: ['local-network'],
    };
  },
};
