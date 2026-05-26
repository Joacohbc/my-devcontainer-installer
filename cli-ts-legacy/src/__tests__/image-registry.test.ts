import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import {
  computeFingerprint,
  fingerprintTag,
  loadRegistry,
  recordEntry,
  removeEntry,
  findByFingerprint,
  saveRegistry,
  imageRegistryPath,
} from '@/domain/image-registry.js';

function withTempHome<T>(fn: () => T): T {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-cli-imgreg-'));
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

const DF = `FROM ubuntu:24.04
RUN apt-get update
LABEL foo=bar
LABEL multi="line" \\
  more=stuff
RUN echo hello
`;

test('computeFingerprint stable for identical inputs', () => {
  const a = computeFingerprint(DF, { 'a.sh': 'echo a' }, ['nodejs', 'go']);
  const b = computeFingerprint(DF, { 'a.sh': 'echo a' }, ['go', 'nodejs']);
  assert.equal(a, b);
});

test('computeFingerprint ignores LABEL-only diffs', () => {
  const a = computeFingerprint(DF, {}, []);
  const stripped = DF.replace('LABEL foo=bar', 'LABEL foo=baz');
  const b = computeFingerprint(stripped, {}, []);
  assert.equal(a, b);
});

test('computeFingerprint changes when copyFile content changes', () => {
  const a = computeFingerprint(DF, { 'a.sh': 'echo a' }, []);
  const b = computeFingerprint(DF, { 'a.sh': 'echo DIFFERENT' }, []);
  assert.notEqual(a, b);
});

test('computeFingerprint changes when Dockerfile body changes', () => {
  const a = computeFingerprint(DF, {}, []);
  const b = computeFingerprint(DF + '\nRUN echo extra', {}, []);
  assert.notEqual(a, b);
});

test('fingerprintTag uses first 12 hex chars', () => {
  const t = fingerprintTag('0123456789abcdef0123456789abcdef');
  assert.equal(t, 'devcontainer-cli/0123456789ab:latest');
});

test('recordEntry upserts by projectDir', () => {
  withTempHome(() => {
    const e = {
      projectDir: '/tmp/proj-1',
      workspace: 'proj1',
      mode: 'remote' as const,
      image: 'ghcr.io/x/y:latest',
      createdAt: '2026-05-22T00:00:00Z',
      lastUpdated: '2026-05-22T00:00:00Z',
    };
    recordEntry(e);
    recordEntry({ ...e, lastUpdated: '2026-05-22T01:00:00Z' });
    const entries = loadRegistry();
    assert.equal(entries.length, 1);
    assert.equal(entries[0].lastUpdated, '2026-05-22T01:00:00Z');
  });
});

test('removeEntry no-ops on missing', () => {
  withTempHome(() => {
    assert.equal(removeEntry('/nonexistent'), false);
    recordEntry({
      projectDir: '/tmp/p',
      workspace: 'p',
      mode: 'local-cached',
      image: 'p:local',
      createdAt: 'now',
      lastUpdated: 'now',
    });
    assert.equal(removeEntry('/tmp/p'), true);
    assert.equal(loadRegistry().length, 0);
  });
});

test('findByFingerprint returns matching entry', () => {
  withTempHome(() => {
    recordEntry({
      projectDir: '/tmp/a',
      workspace: 'a',
      mode: 'local-cached',
      image: 'devcontainer-cli/abc123:latest',
      fingerprint: 'abc123def456',
      createdAt: 'now',
      lastUpdated: 'now',
    });
    const hit = findByFingerprint('abc123def456');
    assert.ok(hit);
    assert.equal(hit.workspace, 'a');
    assert.equal(findByFingerprint('nope'), undefined);
  });
});

test('loadRegistry returns [] when file missing', () => {
  withTempHome(() => {
    assert.deepEqual(loadRegistry(), []);
  });
});

test('saveRegistry creates parent directory', () => {
  withTempHome(() => {
    saveRegistry([
      {
        projectDir: '/x',
        workspace: 'x',
        mode: 'remote',
        image: 'i',
        createdAt: 'now',
        lastUpdated: 'now',
      },
    ]);
    assert.ok(fs.existsSync(imageRegistryPath()));
  });
});
