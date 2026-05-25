import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { defaultConfig, loadConfig, saveConfig } from '@/domain/config.js';
import { CONFIG_FILE } from '@/core/types.js';

function tempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'dc-cli-config-'));
}

test('defaultConfig returns mode=local-cached', () => {
  const cwd = tempDir();
  try {
    assert.equal(defaultConfig(cwd).mode, 'local-cached');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('loadConfig returns null when file missing', () => {
  const cwd = tempDir();
  try {
    assert.equal(loadConfig(cwd), null);
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('loadConfig upgrades legacy mode=custom to local-cached', () => {
  const cwd = tempDir();
  try {
    fs.writeFileSync(
      path.join(cwd, CONFIG_FILE),
      JSON.stringify({
        image: 'legacy:local',
        workspace: 'legacy',
        dockerfile: { modules: [] },
        compose: { services: [], subnet: '172.25.0.0/28' },
        env: {},
      }),
    );
    const loaded = loadConfig(cwd);
    assert.ok(loaded);
    assert.equal(loaded.mode, 'local-cached');
    assert.equal(loaded.image, 'legacy:local');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('loadConfig upgrades explicit mode=custom to local-cached', () => {
  const cwd = tempDir();
  try {
    fs.writeFileSync(
      path.join(cwd, CONFIG_FILE),
      JSON.stringify({
        mode: 'custom',
        image: 'myproject:local',
        workspace: 'myproject',
        dockerfile: { modules: [] },
        compose: { services: [] },
        env: {},
      }),
    );
    const loaded = loadConfig(cwd);
    assert.ok(loaded);
    assert.equal(loaded.mode, 'local-cached');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('loadConfig preserves mode=remote', () => {
  const cwd = tempDir();
  try {
    fs.writeFileSync(
      path.join(cwd, CONFIG_FILE),
      JSON.stringify({
        mode: 'remote',
        image: 'ghcr.io/x/y:latest',
        workspace: 'x',
        dockerfile: { modules: [] },
        compose: { services: [] },
        env: {},
        remote: { variant: 'python' },
      }),
    );
    const loaded = loadConfig(cwd);
    assert.ok(loaded);
    assert.equal(loaded.mode, 'remote');
    assert.equal(loaded.remote?.variant, 'python');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('saveConfig + loadConfig roundtrip preserves remote config', () => {
  const cwd = tempDir();
  try {
    saveConfig(
      {
        mode: 'remote',
        image: 'ghcr.io/joacohbc/devcontainer-python:latest',
        workspace: 'demo',
        dockerfile: { modules: [] },
        compose: { services: [] },
        env: {},
        remote: { variant: 'python' },
      },
      cwd,
    );
    const loaded = loadConfig(cwd);
    assert.ok(loaded);
    assert.equal(loaded.mode, 'remote');
    assert.equal(loaded.remote?.variant, 'python');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});
