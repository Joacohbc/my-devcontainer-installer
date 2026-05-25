import type { ComposeService } from '@/types.js';

// Rootless Docker-in-Docker engine. Runs its own dockerd in an isolated
// sidecar so workloads launched from the devcontainer never touch the host
// daemon. The host Docker socket is never mounted anywhere.
//
// The engine listens on plaintext TCP 2375 — that is acceptable ONLY because
// it lives on a dedicated bridge network (`engine-network`) joined exclusively
// by the devcontainer and this engine. Database/other sidecars stay off that
// network, so they cannot reach the engine.
export const DIND_ENGINE_HOST = 'docker-dind';
export const DIND_ENGINE_PORT = 2375;
export const DIND_ENGINE_NETWORK = 'engine-network';
// Pin the engine image; ':latest' would make generated stacks non-reproducible.
export const DIND_IMAGE = 'docker:28-dind-rootless';

export const dindEngineService: ComposeService = {
  id: DIND_ENGINE_HOST,
  label: 'Rootless Docker-in-Docker engine (isolated sandbox for the "dind" access mode)',
  requiresModule: 'dod',
  volumes: ['dind_data'],
  render() {
    return {
      image: DIND_IMAGE,
      container_name: DIND_ENGINE_HOST,
      restart: 'unless-stopped',
      // Rootless dind still requires privileged to set up cgroups / overlayfs /
      // netns. The "rootless" part means dockerd and the workloads it spawns run
      // as an unprivileged user, so a workload escape lands in a user namespace
      // rather than as host root.
      privileged: true,
      environment: [
        // Empty cert dir => listen on plaintext tcp://0.0.0.0:2375. TLS is
        // intentionally skipped; the engine-network segmentation is the control.
        'DOCKER_TLS_CERTDIR=',
      ],
      volumes: ['dind_data:/home/rootless/.local/share/docker'],
      networks: [DIND_ENGINE_NETWORK],
    };
  },
};
