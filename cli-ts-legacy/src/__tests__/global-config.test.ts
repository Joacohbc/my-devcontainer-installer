import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import {
  loadGlobalConfig,
  saveGlobalConfig,
  resolveRegistry,
  globalConfigPath,
  globalConfigDir,
  REGISTRY_DEFAULT,
} from '@/domain/global-config.js';

function withTempHome<T>(fn: () => T): T {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-cli-globalcfg-'));
  const origXdg = process.env.XDG_CONFIG_HOME;
  const origAppData = process.env.APPDATA;
  process.env.XDG_CONFIG_HOME = tmp;
  process.env.APPDATA = tmp;
  try {
    return fn();
  } finally {
    process.env.XDG_CONFIG_HOME = origXdg;
    process.env.APPDATA = origAppData;
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

test('loadGlobalConfig returns empty object when file missing', () => {
  withTempHome(() => {
    const cfg = loadGlobalConfig();
    assert.deepEqual(cfg, {});
  });
});

test('saveGlobalConfig + loadGlobalConfig roundtrip', () => {
  withTempHome(() => {
    saveGlobalConfig({ registry: 'ghcr.io/foo/' });
    const cfg = loadGlobalConfig();
    assert.equal(cfg.registry, 'ghcr.io/foo/');
  });
});

test('saveGlobalConfig creates parent directory', () => {
  withTempHome(() => {
    saveGlobalConfig({ registry: 'ghcr.io/bar/' });
    assert.ok(fs.existsSync(globalConfigPath()));
    assert.ok(fs.existsSync(globalConfigDir()));
  });
});

test('resolveRegistry default returns ghcr.io/joacohbc/', () => {
  withTempHome(() => {
    assert.equal(resolveRegistry(), REGISTRY_DEFAULT);
    assert.equal(REGISTRY_DEFAULT, 'ghcr.io/joacohbc/');
  });
});

test('resolveRegistry: flag override wins over per-project and global', () => {
  withTempHome(() => {
    saveGlobalConfig({ registry: 'ghcr.io/global/' });
    const got = resolveRegistry('ghcr.io/flag/', 'ghcr.io/proj/');
    assert.equal(got, 'ghcr.io/flag/');
  });
});

test('resolveRegistry: per-project overrides global', () => {
  withTempHome(() => {
    saveGlobalConfig({ registry: 'ghcr.io/global/' });
    assert.equal(resolveRegistry(undefined, 'ghcr.io/proj/'), 'ghcr.io/proj/');
  });
});

test('resolveRegistry: global used when no flag/per-project', () => {
  withTempHome(() => {
    saveGlobalConfig({ registry: 'ghcr.io/global/' });
    assert.equal(resolveRegistry(), 'ghcr.io/global/');
  });
});

test('resolveRegistry: trailing slash normalized', () => {
  withTempHome(() => {
    assert.equal(resolveRegistry('ghcr.io/foo'), 'ghcr.io/foo/');
    assert.equal(resolveRegistry('ghcr.io/foo/'), 'ghcr.io/foo/');
  });
});

test('loadGlobalConfig tolerates corrupt JSON', () => {
  withTempHome(() => {
    fs.mkdirSync(globalConfigDir(), { recursive: true });
    fs.writeFileSync(globalConfigPath(), '{not json}');
    assert.deepEqual(loadGlobalConfig(), {});
  });
});
