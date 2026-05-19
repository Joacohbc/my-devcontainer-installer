import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { preflight } from '../preflight.js';

function mktmp(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'preflight-'));
}

test('preflight (disk fallback) copies real asset from cli/assets into cwd', () => {
  const cwd = mktmp();
  try {
    // entrypoint.sh exists in cli/assets; the on-disk lookup resolves
    // ../assets from src/preflight.ts and finds it.
    const result = preflight(['entrypoint.sh'], cwd);
    assert.deepEqual(result.missing, []);
    assert.deepEqual(result.alreadyPresent, []);
    assert.deepEqual(result.copied, ['entrypoint.sh']);
    assert.ok(fs.existsSync(path.join(cwd, 'entrypoint.sh')));
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('preflight reports already-present files without overwriting', () => {
  const cwd = mktmp();
  try {
    const target = path.join(cwd, 'entrypoint.sh');
    fs.writeFileSync(target, '#!/bin/sh\n# user file\n');
    const result = preflight(['entrypoint.sh'], cwd);
    assert.deepEqual(result.alreadyPresent, ['entrypoint.sh']);
    assert.deepEqual(result.copied, []);
    assert.equal(fs.readFileSync(target, 'utf8'), '#!/bin/sh\n# user file\n');
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});

test('preflight reports missing files when neither SEA nor disk has them', () => {
  const cwd = mktmp();
  try {
    const result = preflight(['does-not-exist.sh'], cwd);
    assert.deepEqual(result.missing, ['does-not-exist.sh']);
    assert.deepEqual(result.copied, []);
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
});
