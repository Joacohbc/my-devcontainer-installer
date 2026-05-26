import type { ComposeService } from '@/core/types.js';
import { SSH_DEFAULTS } from '@/infra/ssh-defaults.js';
import {
  DIND_ENGINE_HOST,
  DIND_ENGINE_PORT,
  DIND_ENGINE_NETWORK,
} from '@/modules/compose/dind-engine.js';

export type DockerSocketMode = 'none' | 'socket' | 'dind';

export const HOST_DOCKER_SOCKET = '/var/run/docker.sock';

const SOCKET_CHOICES: { value: DockerSocketMode; label: string }[] = [
  { value: 'none', label: 'none — no Docker access (safest)' },
  {
    value: 'socket',
    label: 'socket — mount the host /var/run/docker.sock (full host Docker, root-equivalent ⚠️)',
  },
  {
    value: 'dind',
    label: 'dind — isolated rootless Docker-in-Docker engine (sandbox)',
  },
];

export const devcontainerService: ComposeService = {
  id: 'devcontainer',
  label: `${SSH_DEFAULTS.serviceName} (main)`,
  always: true,
  options: [
    {
      id: 'dockerSocket',
      label: 'Docker access mode',
      type: 'select',
      choices: SOCKET_CHOICES,
      default: 'none',
      requiresModule: 'dod',
    },
  ],
  render({ imageName, enabledServiceIds, options }) {
    const depends = enabledServiceIds.filter((s) =>
      ['mongo', 'redis', 'postgres'].includes(s),
    );
    // The dind engine is auto-provisioned by generateCompose whenever this mode
    // is 'dind', so the engine service is always present when we wire DOCKER_HOST.
    const mode = (options.dockerSocket as DockerSocketMode | undefined) ?? 'none';

    const volumes: string[] = [
      '../..:/workspace',
      'devcontainer_etc:/etc',
      'devcontainer_root:/root',
      'devcontainer_home:/home',
    ];
    if (mode === 'socket') volumes.push(`${HOST_DOCKER_SOCKET}:${HOST_DOCKER_SOCKET}`);

    const networks: string[] = ['local-network'];
    if (mode === 'dind') networks.push(DIND_ENGINE_NETWORK);

    const svc: Record<string, unknown> = {
      image: imageName,
      build: '.',
      container_name: SSH_DEFAULTS.serviceName,
      command: 'sleep infinity',
      restart: 'unless-stopped',
      volumes,
      networks,
    };

    if (mode === 'dind') {
      svc.environment = [`DOCKER_HOST=tcp://${DIND_ENGINE_HOST}:${DIND_ENGINE_PORT}`];
    }

    const extraDepends = mode === 'dind' ? [DIND_ENGINE_HOST] : [];
    const allDepends = [...depends, ...extraDepends];
    if (allDepends.length > 0) svc.depends_on = allDepends;
    return svc;
  },
};
