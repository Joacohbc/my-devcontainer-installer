import type { ComposeService } from '@/types.js';
import { SSH_DEFAULTS } from '@/ssh-defaults.js';
import {
  DIND_ENGINE_HOST,
  DIND_ENGINE_PORT,
  DIND_ENGINE_NETWORK,
} from '@/modules/compose/dind-engine.js';

export type DockerSocketMode = 'none' | 'dind';

const SOCKET_CHOICES: { value: DockerSocketMode; label: string }[] = [
  { value: 'none', label: 'none — no Docker access (safest)' },
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
    },
  ],
  render({ imageName, enabledServiceIds, options }) {
    const depends = enabledServiceIds.filter((s) =>
      ['mongo', 'redis', 'postgres'].includes(s),
    );
    const requested = (options.dockerSocket as DockerSocketMode | undefined) ?? 'none';
    const dindEnabled = enabledServiceIds.includes(DIND_ENGINE_HOST);

    // Fail safe: if dind is requested but the engine service is not enabled,
    // fall back to 'none' (no access) rather than producing a broken DOCKER_HOST.
    const mode: DockerSocketMode = requested === 'dind' && dindEnabled ? 'dind' : 'none';

    const volumes: string[] = [
      '../..:/workspace',
      'devcontainer_etc:/etc',
      'devcontainer_root:/root',
      'devcontainer_home:/home',
    ];

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
