import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseDownFlags, downHelp } from '../down.js';

test('parseDownFlags: defaults', () => {
  const f = parseDownFlags([]);
  assert.equal(f.yes, false);
  assert.equal(f.help, false);
  assert.equal(f.interactive, true);
});

test('parseDownFlags: --yes', () => {
  assert.equal(parseDownFlags(['--yes']).yes, true);
  assert.equal(parseDownFlags(['-y']).yes, true);
});

test('parseDownFlags: --help', () => {
  assert.equal(parseDownFlags(['--help']).help, true);
  assert.equal(parseDownFlags(['-h']).help, true);
});

test('parseDownFlags: --no-interactive', () => {
  assert.equal(parseDownFlags(['--no-interactive']).interactive, false);
});

test('parseDownFlags: unknown flag throws', () => {
  assert.throws(() => parseDownFlags(['--banana']));
});

test('downHelp mentions compose down -v', () => {
  assert.match(downHelp(), /down -v/);
});
