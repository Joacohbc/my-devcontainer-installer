import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse } from 'yaml';
import {
  generateCompose as generateComposeRaw,
  generateDockerfile as generateDockerfileRaw,
  generateEnv,
  resolveRemoteImage,
} from '@/domain/generator.js';
import type { DevcontainerConfig } from '@/core/types.js';

function generateDockerfile(config: DevcontainerConfig): string {
  const out = generateDockerfileRaw(config);
  if (out === null) throw new Error('generateDockerfile returned null');
  return out;
}

function generateCompose(config: DevcontainerConfig): string {
  const out = generateComposeRaw(config);
  if (out === null) throw new Error('generateCompose returned null');
  return out;
}

function makeConfig(overrides: Partial<DevcontainerConfig> = {}): DevcontainerConfig {
  return {
    mode: 'local-cached',
    image: 'devcontainer-ssh:local',
    workspace: 'devcontainer',
    dockerfile: { modules: [] },
    compose: { services: [], subnet: '172.25.0.0/24' },
    env: {},
    ...overrides,
  };
}

test('minimal Dockerfile has base + cleanup', () => {
  const df = generateDockerfile(makeConfig());
  assert.match(df, /FROM ubuntu:24\.04/);
  assert.match(df, /ENTRYPOINT \["\/entrypoint\.sh"\]/);
  assert.match(df, /AUTO-GENERATED/);
});

test('full Dockerfile contains all selected modules', () => {
  const df = generateDockerfile(
    makeConfig({
      dockerfile: {
        modules: [
          { id: 'dod' },
          { id: 'java-temurin' },
          { id: 'python' },
          { id: 'sqlite' },
          { id: 'go' },
          { id: 'dbclients' },
          { id: 'nodejs' },
          { id: 'bun' },
          { id: 'tmux' },
        ],
      },
    }),
  );
  assert.match(df, /docker-ce-cli/);
  assert.match(df, /temurin-17-jdk/);
  assert.match(df, /python3-pip/);
  assert.match(df, /sqlite3/);
  assert.match(df, /install_golang/);
  assert.match(df, /mongodb-mongosh/);
  assert.match(df, /nvm install --lts/);
  assert.match(df, /bun\.sh\/install/);
  assert.match(df, /tmux/);
});

test('tmux module renders successfully', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'tmux' }] } }),
  );
  assert.match(df, /RUN apt-get update && apt-get install -y tmux/);
});

test('python module includes uv by default', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'python' }] } }),
  );
  assert.match(df, /astral\.sh\/uv\/install\.sh/);
  // uv PATH is sourced from every rc file (not just .profile, unread by zsh)
  assert.match(df, /\.python_init\.sh/);
  assert.match(df, /for f in \.zshrc \.bashrc \.profile/);
});

test('bun module sources its PATH from every rc file', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'bun' }] } }),
  );
  assert.match(df, /bun\.sh\/install/);
  assert.match(df, /\.bun_init\.sh/);
  assert.match(df, /for f in \.zshrc \.bashrc \.profile/);
});

test('python without uv', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'python', options: { uv: false } }] } }),
  );
  assert.doesNotMatch(df, /astral\.sh/);
});

test('pnpm module auto-pulls nodejs (requires)', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'pnpm' }] } }),
  );
  assert.match(df, /nvm install/);
  assert.match(df, /get\.pnpm\.io\/install\.sh/);
});

test('nodejs with fnm manager', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'nodejs', options: { manager: 'fnm' } }] } }),
  );
  assert.match(df, /fnm\.vercel\.app\/install/);
  assert.match(df, /fnm install --lts/);
  assert.doesNotMatch(df, /nvm-sh\/nvm/);
});

test('nodejs with fnm + specific version', () => {
  const df = generateDockerfile(
    makeConfig({
      dockerfile: { modules: [{ id: 'nodejs', options: { manager: 'fnm', version: '22' } }] },
    }),
  );
  assert.match(df, /fnm install 22/);
});

test('compose YAML is valid and only contains enabled services', () => {
  const yml = generateCompose(
    makeConfig({ compose: { services: ['mongo', 'tunnel'], subnet: '10.0.0.0/24' } }),
  );
  const parsed = parse(yml) as { services: Record<string, unknown> };
  assert.ok(parsed.services['devcontainer-ssh']);
  assert.ok(parsed.services.mongo);
  assert.ok(parsed.services.tunnel);
  assert.equal(parsed.services.redis, undefined);
  assert.equal(parsed.services.postgres, undefined);
});

test('compose depends_on only references enabled DB services', () => {
  const yml = generateCompose(
    makeConfig({ compose: { services: ['mongo'], subnet: '172.25.0.0/24' } }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { depends_on?: string[] } };
  };
  assert.deepEqual(parsed.services['devcontainer-ssh'].depends_on, ['mongo']);
});

test('compose with no DB services has no depends_on', () => {
  const yml = generateCompose(makeConfig({ compose: { services: [], subnet: '172.25.0.0/24' } }));
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { depends_on?: string[] } };
  };
  assert.equal(parsed.services['devcontainer-ssh'].depends_on, undefined);
});

test('compose assigns a static IP to devcontainer based on subnet', () => {
  const yml = generateCompose(
    makeConfig({ compose: { services: [], subnet: '172.25.0.0/28' } }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { networks: Record<string, { ipv4_address: string }> } };
    networks: Record<string, { ipam: { config: { subnet: string }[] } }>;
  };
  const networkName = Object.keys(parsed.services['devcontainer-ssh'].networks)[0];
  assert.equal(
    parsed.services['devcontainer-ssh'].networks[networkName].ipv4_address,
    '${DEVCONTAINER_IP:-172.25.0.14}',
  );
  assert.match(parsed.networks[networkName].ipam.config[0].subnet, /172\.25\.0\.0\/28/);
});

test('ai-clis module ships per-tool install scripts as post-scripts by default', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'ai-clis' }] } }),
  );
  assert.match(df, /COPY .*install-claude-code\.sh.*install-opencode\.sh.*install-codex-cli\.sh.*install-antigravity\.sh.*install-copilot\.sh.*\/home\/devuser\/post-script\//);
  assert.match(df, /chmod \+x \/home\/devuser\/post-script\/\*\.sh/);
  assert.doesNotMatch(df, /ai-login/);
});

test('ai-clis module respects tools subset', () => {
  const df = generateDockerfile(
    makeConfig({
      dockerfile: {
        modules: [{ id: 'ai-clis', options: { tools: ['claude-code'] } }],
      },
    }),
  );
  assert.match(df, /install-claude-code\.sh/);
  assert.doesNotMatch(df, /install-opencode\.sh/);
  assert.doesNotMatch(df, /install-codex-cli\.sh/);
  assert.doesNotMatch(df, /install-antigravity\.sh/);
  assert.doesNotMatch(df, /install-copilot\.sh/);
});

test('ai-clis pulls pnpm + github-cli via requires', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'ai-clis' }] } }),
  );
  assert.match(df, /get\.pnpm\.io\/install\.sh/);
  assert.match(df, /cli\.github\.com\/packages/);
});

test('compose default does not mount the docker socket into devcontainer', () => {
  const yml = generateCompose(makeConfig());
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { volumes: string[]; environment?: string[] } };
  };
  const vols = parsed.services['devcontainer-ssh'].volumes;
  assert.ok(!vols.some((v) => v.includes('docker.sock')), 'socket must not be mounted by default');
  assert.equal(parsed.services['devcontainer-ssh'].environment, undefined);
});

test('compose dockerSocket=dind auto-provisions the engine and wires DOCKER_HOST', () => {
  // The dind engine is NOT listed explicitly — it must be auto-provisioned
  // purely from the devcontainer's access mode.
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [
          { id: 'devcontainer', options: { dockerSocket: 'dind' } },
          { id: 'postgres', options: {} },
        ],
        subnet: '172.25.0.0/24',
      },
    }),
  );
  const parsed = parse(yml) as {
    services: {
      'devcontainer-ssh': {
        volumes: string[];
        environment: string[];
        depends_on: string[];
        networks: Record<string, unknown>;
      };
      'docker-dind': {
        image: string;
        privileged: boolean;
        volumes: string[];
        networks: string[];
      };
      postgres: { networks: string[] | Record<string, unknown> };
    };
    networks: Record<string, unknown>;
  };
  const dev = parsed.services['devcontainer-ssh'];
  // No host socket is ever mounted in dind mode.
  assert.ok(!dev.volumes.some((v) => v.includes('docker.sock')));
  assert.ok(dev.environment.includes('DOCKER_HOST=tcp://docker-dind:2375'));
  assert.ok(dev.depends_on.includes('docker-dind'));
  // Devcontainer joins both the main and the dedicated engine network.
  assert.ok('devcontainer-network' in dev.networks);
  assert.ok('devcontainer-engine-network' in dev.networks);

  const dind = parsed.services['docker-dind'];
  assert.match(dind.image, /docker:\d+-dind-rootless/);
  assert.equal(dind.privileged, true);
  assert.ok(!dind.volumes.some((v) => v.includes('docker.sock')));
  assert.deepEqual(dind.networks, ['devcontainer-engine-network']);

  // The engine network exists and DB sidecars are kept off it.
  assert.ok('devcontainer-engine-network' in parsed.networks);
  const pg = parsed.services.postgres.networks;
  assert.ok(!JSON.stringify(pg).includes('engine-network'));
});

test('compose without dind mode does not provision the dind engine', () => {
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [{ id: 'devcontainer', options: { dockerSocket: 'none' } }],
        subnet: '172.25.0.0/24',
      },
    }),
  );
  const parsed = parse(yml) as {
    services: Record<string, { volumes: string[]; environment?: string[] }>;
  };
  assert.ok(!('docker-dind' in parsed.services));
  const dev = parsed.services['devcontainer-ssh'];
  assert.ok(!dev.volumes.some((v) => v.includes('docker.sock')));
  assert.equal(dev.environment, undefined);
});

test('compose dockerSocket=socket mounts the host docker socket', () => {
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [{ id: 'devcontainer', options: { dockerSocket: 'socket' } }],
        subnet: '172.25.0.0/24',
      },
    }),
  );
  const parsed = parse(yml) as {
    services: {
      'devcontainer-ssh': { volumes: string[]; environment?: string[]; networks: Record<string, unknown> };
    };
  };
  const dev = parsed.services['devcontainer-ssh'];
  assert.ok(dev.volumes.some((v) => v === '/var/run/docker.sock:/var/run/docker.sock'));
  // Host-socket mode talks to the host daemon directly: no DOCKER_HOST, no engine network.
  assert.equal(dev.environment, undefined);
  assert.ok(!('devcontainer-engine-network' in dev.networks));
});

test('env file includes selected env vars', () => {
  const env = generateEnv(
    makeConfig({ env: { TUNNEL_TOKEN: 'abc' }, compose: { services: [], subnet: '10.0.0.0/8' } }),
  );
  assert.match(env, /TUNNEL_TOKEN=abc/);
  assert.match(env, /DOCKER_SUBNET=10\.0\.0\.0\/8/);
});

test('env file derives DEVCONTAINER_IP from DOCKER_SUBNET override', () => {
  const env = generateEnv(
    makeConfig({
      env: { DOCKER_SUBNET: '172.26.0.0/24' },
      compose: { services: [], subnet: '172.25.0.0/28' },
    }),
  );
  assert.match(env, /DOCKER_SUBNET=172\.26\.0\.0\/24/);
  assert.match(env, /DEVCONTAINER_IP=172\.26\.0\.254/);
});

test('env file writes both DOCKER_SUBNET and DEVCONTAINER_IP from compose subnet', () => {
  const env = generateEnv(
    makeConfig({ compose: { services: [], subnet: '172.25.0.0/28' } }),
  );
  assert.match(env, /DOCKER_SUBNET=172\.25\.0\.0\/28/);
  assert.match(env, /DEVCONTAINER_IP=172\.25\.0\.14/);
});

test('mode=remote returns null Dockerfile', () => {
  assert.equal(
    generateDockerfileRaw(
      makeConfig({
        mode: 'remote',
        remote: { variant: 'python' },
      }),
    ),
    null,
  );
});

test('mode=remote compose omits build and uses ghcr image', () => {
  const yml = generateCompose(
    makeConfig({
      mode: 'remote',
      image: 'ghcr.io/joacohbc/devcontainer-node-java-temurin:latest',
      remote: { variant: 'node-java-temurin' },
    }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { image: string; build?: unknown } };
  };
  assert.equal(parsed.services['devcontainer-ssh'].build, undefined);
  assert.equal(
    parsed.services['devcontainer-ssh'].image,
    'ghcr.io/joacohbc/devcontainer-node-java-temurin:latest',
  );
});

test('mode=remote with DB service still emits compose for the DB', () => {
  const yml = generateCompose(
    makeConfig({
      mode: 'remote',
      image: 'ghcr.io/joacohbc/devcontainer-ssh:latest',
      remote: { variant: 'ssh' },
      compose: { services: ['mongo'], subnet: '172.25.0.0/24' },
    }),
  );
  const parsed = parse(yml) as { services: Record<string, unknown> };
  assert.ok(parsed.services.mongo);
});

test('resolveRemoteImage: ssh variant uses devcontainer-ssh suffix', () => {
  assert.equal(
    resolveRemoteImage('ssh', 'ghcr.io/joacohbc/'),
    'ghcr.io/joacohbc/devcontainer-ssh:latest',
  );
});

test('resolveRemoteImage: non-ssh variant uses devcontainer-<variant>', () => {
  assert.equal(
    resolveRemoteImage('bun', 'ghcr.io/joacohbc/'),
    'ghcr.io/joacohbc/devcontainer-bun:latest',
  );
});

test('resolveRemoteImage: registry override beats per-project', () => {
  assert.equal(
    resolveRemoteImage('python', 'ghcr.io/flag/', 'ghcr.io/proj/'),
    'ghcr.io/flag/devcontainer-python:latest',
  );
});

test('mode=local-cached uses devcontainer-cli/<fp>:latest as image when fingerprint set', () => {
  const yml = generateCompose(
    makeConfig({
      mode: 'local-cached',
      fingerprint: 'abc123def4567890abc123def4567890',
      image: 'should-not-be-used:tag',
    }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { image: string; build?: unknown } };
  };
  assert.equal(parsed.services['devcontainer-ssh'].image, 'devcontainer-cli/abc123def456:latest');
  // build: '.' still present — daemon-level skip is decided at runtime, not in the compose
  assert.equal(parsed.services['devcontainer-ssh'].build, '.');
});
