import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseFlags } from '../cli.js';

test('parses --with as comma list', () => {
  const f = parseFlags(['--with', 'nodejs,java,dod']);
  assert.deepEqual(f.withModules, ['nodejs', 'java', 'dod']);
});

test('--no-interactive flips flag', () => {
  const f = parseFlags(['--no-interactive']);
  assert.equal(f.interactive, false);
});

test('--image captures value', () => {
  const f = parseFlags(['--image', 'foo:bar']);
  assert.equal(f.image, 'foo:bar');
});

test('unknown flag throws', () => {
  assert.throws(() => parseFlags(['--banana']));
});

test('--build / --no-build', () => {
  assert.equal(parseFlags(['--build']).build, true);
  assert.equal(parseFlags(['--no-build']).build, false);
  assert.equal(parseFlags([]).build, null);
});

test('--force flips flag', () => {
  const f = parseFlags(['--force']);
  assert.equal(f.force, true);
});
