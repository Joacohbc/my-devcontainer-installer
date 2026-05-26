import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  isDockerAvailable,
  ensureDocker,
  dockerInherit,
  dockerCapture,
  dockerCompose,
  dockerComposeOrThrow,
  spawner,
  resetDockerCache,
} from '@/infra/docker.js';

test('docker.ts checks availability and executes commands', () => {
  resetDockerCache();
  const original = spawner.spawnSync;
  spawner.spawnSync = ((cmd: string, args: string[], options: any) => {
    return { status: 0, stdout: 'client-version', stderr: '' };
  }) as any;

  try {
    assert.equal(isDockerAvailable(), true);
    assert.doesNotThrow(() => ensureDocker());

    const inheritStatus = dockerInherit(['ps']);
    assert.equal(inheritStatus, 0);

    const captureResult = dockerCapture(['info']);
    assert.equal(captureResult.status, 0);
    assert.equal(captureResult.stdout, 'client-version');

    const composeStatus = dockerCompose('docker-compose.yml', ['up']);
    assert.equal(composeStatus, 0);

    assert.doesNotThrow(() => dockerComposeOrThrow('docker-compose.yml', ['up']));
  } finally {
    spawner.spawnSync = original;
  }
});

test('docker.ts behaves correctly when docker is missing', () => {
  resetDockerCache();
  const original = spawner.spawnSync;
  spawner.spawnSync = ((cmd: string, args: string[], options: any) => {
    return { status: 1, stdout: '', stderr: 'error' };
  }) as any;

  try {
    assert.equal(isDockerAvailable(), false);
    assert.throws(() => ensureDocker());
  } finally {
    spawner.spawnSync = original;
  }
});
