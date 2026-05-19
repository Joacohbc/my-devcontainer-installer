import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse } from 'yaml';
import { generateCompose, generateDockerfile, generateEnv } from '../generator.js';
import type { DevcontainerConfig } from '../types.js';

function makeConfig(overrides: Partial<DevcontainerConfig> = {}): DevcontainerConfig {
  return {
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
});

test('python module includes uv by default', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'python' }] } }),
  );
  assert.match(df, /astral\.sh\/uv\/install\.sh/);
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

test('ai-clis module copies per-tool install scripts by default', () => {
  const df = generateDockerfile(
    makeConfig({ dockerfile: { modules: [{ id: 'ai-clis' }] } }),
  );
  assert.match(df, /COPY install-claude-code\.sh install-gemini\.sh install-opencode\.sh install-autoskills\.sh \/home\/devuser\//);
  assert.match(df, /chmod \+x .*\/home\/devuser\/install-claude-code\.sh/);
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
  assert.doesNotMatch(df, /install-gemini\.sh/);
  assert.doesNotMatch(df, /install-opencode\.sh/);
  assert.doesNotMatch(df, /install-autoskills\.sh/);
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

test('compose dockerSocket=ro mounts socket read-only', () => {
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [{ id: 'devcontainer', options: { dockerSocket: 'ro' } }],
        subnet: '172.25.0.0/24',
      },
    }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { volumes: string[] } };
  };
  assert.ok(
    parsed.services['devcontainer-ssh'].volumes.includes(
      '/var/run/docker.sock:/var/run/docker.sock:ro',
    ),
  );
});

test('compose dockerSocket=proxy wires DOCKER_HOST and depends_on proxy', () => {
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [
          { id: 'devcontainer', options: { dockerSocket: 'proxy' } },
          { id: 'docker-socket-proxy', options: {} },
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
      };
      'docker-socket-proxy': {
        image: string;
        environment: string[];
        volumes: string[];
      };
    };
  };
  const dev = parsed.services['devcontainer-ssh'];
  assert.ok(!dev.volumes.some((v) => v.includes('docker.sock')));
  assert.ok(dev.environment.includes('DOCKER_HOST=tcp://docker-socket-proxy:2375'));
  assert.ok(dev.depends_on.includes('docker-socket-proxy'));
  const proxy = parsed.services['docker-socket-proxy'];
  assert.match(proxy.image, /tecnativa\/docker-socket-proxy/);
  assert.ok(proxy.volumes.includes('/var/run/docker.sock:/var/run/docker.sock:ro'));
  assert.ok(proxy.environment.includes('POST=1'));
  assert.ok(proxy.environment.includes('EXEC=0'));
});

test('compose dockerSocket=proxy without proxy service falls back to ro mount', () => {
  const yml = generateCompose(
    makeConfig({
      compose: {
        services: [{ id: 'devcontainer', options: { dockerSocket: 'proxy' } }],
        subnet: '172.25.0.0/24',
      },
    }),
  );
  const parsed = parse(yml) as {
    services: { 'devcontainer-ssh': { volumes: string[]; environment?: string[] } };
  };
  assert.ok(
    parsed.services['devcontainer-ssh'].volumes.includes(
      '/var/run/docker.sock:/var/run/docker.sock:ro',
    ),
  );
  assert.equal(parsed.services['devcontainer-ssh'].environment, undefined);
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
