import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse } from 'yaml';
import { generateCompose, generateDockerfile, generateEnv } from '../generator.js';
import type { DevcontainerConfig } from '../types.js';

function makeConfig(overrides: Partial<DevcontainerConfig> = {}): DevcontainerConfig {
  return {
    image: 'devcontainer-ssh:local',
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

test('env file includes selected env vars', () => {
  const env = generateEnv(
    makeConfig({ env: { TUNNEL_TOKEN: 'abc' }, compose: { services: [], subnet: '10.0.0.0/8' } }),
  );
  assert.match(env, /TUNNEL_TOKEN=abc/);
  assert.match(env, /DOCKER_SUBNET=10\.0\.0\.0\/8/);
});
