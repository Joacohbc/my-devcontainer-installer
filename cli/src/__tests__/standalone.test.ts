import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as path from 'path';
import { parseStandaloneFlags, generateDockerRunCommand } from '../standalone.js';
import type { DevcontainerConfig } from '../types.js';

test('parses standalone --with as comma list', () => {
  const f = parseStandaloneFlags(['--with', 'nodejs,python']);
  assert.deepEqual(f.withModules, ['nodejs', 'python']);
});

test('parses standalone flags correctly', () => {
  const f = parseStandaloneFlags([
    '--image', 'myimg:latest',
    '--workspace', 'myworkspace',
    '--mount', '/foo/bar',
    '--subnet', '10.0.0.0/24',
    '--no-build',
    '--force',
  ]);
  assert.equal(f.image, 'myimg:latest');
  assert.equal(f.workspace, 'myworkspace');
  assert.equal(f.mount, '/foo/bar');
  assert.equal(f.subnet, '10.0.0.0/24');
  assert.equal(f.build, false);
  assert.equal(f.force, true);
});

test('parses standalone build flag', () => {
  assert.equal(parseStandaloneFlags(['--build']).build, true);
  assert.equal(parseStandaloneFlags(['--no-build']).build, false);
  assert.equal(parseStandaloneFlags([]).build, null);
});

test('generateDockerRunCommand without mount', () => {
  const config: DevcontainerConfig = {
    image: 'testimg:local',
    workspace: 'myws',
    mode: 'standalone',
    dockerfile: { modules: [] },
    compose: { services: [] },
    standalone: {
      subnet: '172.30.0.0/28',
    },
    env: {},
  };

  const { networkCommand, runCommand } = generateDockerRunCommand(config);
  assert.equal(networkCommand, 'docker network create --subnet 172.30.0.0/28 myws-network');
  assert.match(runCommand, /docker run -d/);
  assert.match(runCommand, /--name myws-devcontainer-ssh/);
  assert.match(runCommand, /--network myws-network/);
  assert.match(runCommand, /--ip 172.30.0.14/);
  assert.match(runCommand, /-v \/var\/run\/docker\.sock:\/var\/run\/docker\.sock/);
  assert.doesNotMatch(runCommand, /-v .*:\/workspace/);
  assert.match(runCommand, /testimg:local/);
  assert.match(runCommand, /sleep infinity/);
});

test('generateDockerRunCommand with mount', () => {
  const config: DevcontainerConfig = {
    image: 'testimg:local',
    workspace: 'myws',
    mode: 'standalone',
    dockerfile: { modules: [] },
    compose: { services: [] },
    standalone: {
      subnet: '172.30.0.0/28',
      mount: '/some/host/path',
    },
    env: {},
  };

  const { runCommand } = generateDockerRunCommand(config);
  const hostPath = path.resolve('/some/host/path');
  assert.match(runCommand, new RegExp(`-v ${hostPath}:/workspace`));
});
