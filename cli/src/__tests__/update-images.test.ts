import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { parseUpdateFlags, updateHelp } from '../update-images.js';

test('parseUpdateFlags: --all', () => {
  assert.equal(parseUpdateFlags(['--all']).all, true);
});

test('parseUpdateFlags: --pull / --rebuild', () => {
  const f = parseUpdateFlags(['--pull', '--rebuild']);
  assert.equal(f.pull, true);
  assert.equal(f.rebuild, true);
});

test('parseUpdateFlags: rejects unknown flag', () => {
  assert.throws(() => parseUpdateFlags(['--banana']));
});

test('parseUpdateFlags: --help', () => {
  assert.equal(parseUpdateFlags(['--help']).help, true);
  assert.equal(parseUpdateFlags(['-h']).help, true);
});

test('updateHelp mentions upgrade-cli for binary self-update', () => {
  const text = updateHelp();
  assert.match(text, /upgrade-cli/);
  assert.match(text, /update --all/);
});
