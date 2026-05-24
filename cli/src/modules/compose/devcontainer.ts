import type { ComposeService } from '../../types.js';
import { SSH_DEFAULTS } from '../../ssh-defaults.js';
import {
  DOCKER_SOCKET_PROXY_HOST,
  DOCKER_SOCKET_PROXY_PORT,
} from './docker-socket-proxy.js';
import {
  DIND_ENGINE_HOST,
  DIND_ENGINE_PORT,
  DIND_ENGINE_NETWORK,
} from './dind-engine.js';

export type DockerSocketMode = 'none' | 'dind' | 'proxy' | 'rw';

const SOCKET_CHOICES: { value: DockerSocketMode; label: string }[] = [
  { value: 'none', label: 'none — no Docker access (safest)' },
  {
    value: 'dind',
    label: 'dind — isolated rootless Docker-in-Docker engine (recommended sandbox)',
  },
  {
    value: 'proxy',
    label: 'proxy — host daemon via docker-socket-proxy (hardened DooD, not a sandbox)',
  },
  { value: 'rw', label: 'rw — direct read-write host socket mount (= host root, unsafe)' },
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
    },
  ],
  render({ imageName, enabledServiceIds, options }) {
    const depends = enabledServiceIds.filter((s) =>
      ['mongo', 'redis', 'postgres'].includes(s),
    );
    const requested = (options.dockerSocket as DockerSocketMode | undefined) ?? 'none';
    const proxyEnabled = enabledServiceIds.includes(DOCKER_SOCKET_PROXY_HOST);
    const dindEnabled = enabledServiceIds.includes(DIND_ENGINE_HOST);

    // Fail safe, never down to a weaker-but-silent mode: if the backing service
    // for the requested mode is not enabled, fall back to 'none' (no access)
    // rather than silently mounting the host socket.
    let mode: DockerSocketMode = requested;
    if (mode === 'proxy' && !proxyEnabled) mode = 'none';
    if (mode === 'dind' && !dindEnabled) mode = 'none';

    const volumes: string[] = [
      '../..:/workspace',
      'devcontainer_etc:/etc',
      'devcontainer_root:/root',
      'devcontainer_home:/home',
    ];
    if (mode === 'rw') {
      volumes.splice(1, 0, '/var/run/docker.sock:/var/run/docker.sock');
    }

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

    if (mode === 'proxy') {
      svc.environment = [
        `DOCKER_HOST=tcp://${DOCKER_SOCKET_PROXY_HOST}:${DOCKER_SOCKET_PROXY_PORT}`,
      ];
    } else if (mode === 'dind') {
      svc.environment = [`DOCKER_HOST=tcp://${DIND_ENGINE_HOST}:${DIND_ENGINE_PORT}`];
    }

    const extraDepends =
      mode === 'proxy'
        ? [DOCKER_SOCKET_PROXY_HOST]
        : mode === 'dind'
          ? [DIND_ENGINE_HOST]
          : [];
    const allDepends = [...depends, ...extraDepends];
    if (allDepends.length > 0) svc.depends_on = allDepends;
    return svc;
  },
};
