import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseDestroyFlags, destroyHelp } from '@/destroy.js';

test('parseDestroyFlags: defaults', () => {
  const f = parseDestroyFlags([]);
  assert.equal(f.yes, false);
  assert.equal(f.help, false);
  assert.equal(f.interactive, true);
});

test('parseDestroyFlags: --yes', () => {
  assert.equal(parseDestroyFlags(['--yes']).yes, true);
  assert.equal(parseDestroyFlags(['-y']).yes, true);
});

test('parseDestroyFlags: --help', () => {
  assert.equal(parseDestroyFlags(['--help']).help, true);
  assert.equal(parseDestroyFlags(['-h']).help, true);
});

test('parseDestroyFlags: --no-interactive', () => {
  assert.equal(parseDestroyFlags(['--no-interactive']).interactive, false);
});

test('parseDestroyFlags: unknown flag throws', () => {
  assert.throws(() => parseDestroyFlags(['--banana']));
});

test('destroyHelp warns it is irreversible', () => {
  assert.match(destroyHelp(), /irreversible/);
});
