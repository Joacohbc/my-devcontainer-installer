import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { runConfigCmd } from '@/commands/config-cmd.js';
import { loadGlobalConfig } from '@/domain/global-config.js';

function withTempHome<T>(fn: () => Promise<T>): Promise<T> {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-cli-configcmd-'));
  const origXdg = process.env.XDG_CONFIG_HOME;
  const origAppData = process.env.APPDATA;
  process.env.XDG_CONFIG_HOME = tmp;
  process.env.APPDATA = tmp;
  return fn().finally(() => {
    process.env.XDG_CONFIG_HOME = origXdg;
    process.env.APPDATA = origAppData;
    fs.rmSync(tmp, { recursive: true, force: true });
  });
}

function silentConsole<T>(fn: () => Promise<T>): Promise<T> {
  const origLog = console.log;
  console.log = () => undefined;
  return fn().finally(() => {
    console.log = origLog;
  });
}

test('config registry <url> persists value', async () => {
  await withTempHome(async () => {
    await silentConsole(() => runConfigCmd(['registry', 'ghcr.io/foo/']));
    assert.equal(loadGlobalConfig().registry, 'ghcr.io/foo/');
  });
});

test('config registry (no value) prints current', async () => {
  await withTempHome(async () => {
    await silentConsole(() => runConfigCmd(['registry', 'ghcr.io/bar/']));
    await silentConsole(() => runConfigCmd(['registry']));
    assert.equal(loadGlobalConfig().registry, 'ghcr.io/bar/');
  });
});

test('config registry --unset clears value', async () => {
  await withTempHome(async () => {
    await silentConsole(() => runConfigCmd(['registry', 'ghcr.io/baz/']));
    await silentConsole(() => runConfigCmd(['registry', '--unset']));
    assert.equal(loadGlobalConfig().registry, undefined);
  });
});

test('config <unknown> throws', async () => {
  await withTempHome(async () => {
    await assert.rejects(() => runConfigCmd(['banana']));
  });
});

test('config (no args) prints help', async () => {
  await withTempHome(async () => {
    await silentConsole(() => runConfigCmd([]));
  });
});
