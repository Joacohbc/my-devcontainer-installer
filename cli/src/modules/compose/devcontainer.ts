import type { ComposeService } from '../../types.js';
import { SSH_DEFAULTS } from '../../ssh-defaults.js';
import {
  DOCKER_SOCKET_PROXY_HOST,
  DOCKER_SOCKET_PROXY_PORT,
} from './docker-socket-proxy.js';

export type DockerSocketMode = 'none' | 'proxy' | 'ro' | 'rw';

const SOCKET_CHOICES: { value: DockerSocketMode; label: string }[] = [
  { value: 'none', label: 'none — no Docker socket access (safest)' },
  { value: 'proxy', label: 'proxy — via docker-socket-proxy service (recommended for DooD)' },
  { value: 'ro', label: 'ro — direct read-only socket mount' },
  { value: 'rw', label: 'rw — direct read-write socket mount (legacy, unsafe)' },
];

export const devcontainerService: ComposeService = {
  id: 'devcontainer',
  label: `${SSH_DEFAULTS.serviceName} (main)`,
  always: true,
  options: [
    {
      id: 'dockerSocket',
      label: 'Docker socket access mode',
      type: 'select',
      choices: SOCKET_CHOICES,
      default: 'none',
    },
  ],
  render({ imageName, enabledServiceIds, options }) {
    const depends = enabledServiceIds.filter((s) =>
      ['mongo', 'redis', 'postgres'].includes(s),
    );
    const requested = (options.dockerSocket as DockerSocketMode | undefined) ?? 'none';
    const proxyEnabled = enabledServiceIds.includes(DOCKER_SOCKET_PROXY_HOST);
    let mode: DockerSocketMode = requested;
    if (mode === 'proxy' && !proxyEnabled) {
      mode = 'ro';
    }

    const volumes: string[] = [
      '../..:/workspace',
      'devcontainer_etc:/etc',
      'devcontainer_root:/root',
      'devcontainer_home:/home',
    ];
    if (mode === 'ro') {
      volumes.splice(1, 0, '/var/run/docker.sock:/var/run/docker.sock:ro');
    } else if (mode === 'rw') {
      volumes.splice(1, 0, '/var/run/docker.sock:/var/run/docker.sock');
    }

    const svc: Record<string, unknown> = {
      image: imageName,
      build: '.',
      container_name: SSH_DEFAULTS.serviceName,
      command: 'sleep infinity',
      restart: 'unless-stopped',
      volumes,
      networks: ['local-network'],
    };

    if (mode === 'proxy') {
      svc.environment = [
        `DOCKER_HOST=tcp://${DOCKER_SOCKET_PROXY_HOST}:${DOCKER_SOCKET_PROXY_PORT}`,
      ];
    }

    const allDepends = mode === 'proxy' ? [...depends, DOCKER_SOCKET_PROXY_HOST] : depends;
    if (allDepends.length > 0) svc.depends_on = allDepends;
    return svc;
  },
};
