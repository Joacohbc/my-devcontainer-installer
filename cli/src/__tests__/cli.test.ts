import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseFlags, helpText } from '@/cli.js';

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

test('--mode accepts both build modes', () => {
  assert.equal(parseFlags(['--mode', 'local-cached']).mode, 'local-cached');
  assert.equal(parseFlags(['--mode', 'remote']).mode, 'remote');
});

test('--mode rejects invalid value', () => {
  assert.throws(() => parseFlags(['--mode', 'banana']));
  assert.throws(() => parseFlags(['--mode', 'custom']));
  assert.throws(() => parseFlags(['--mode', 'standalone']));
});

test('--variant accepts ssh + all variants', () => {
  assert.equal(parseFlags(['--variant', 'ssh']).variant, 'ssh');
  assert.equal(parseFlags(['--variant', 'node-java-temurin']).variant, 'node-java-temurin');
  assert.equal(parseFlags(['--variant', 'python']).variant, 'python');
});

test('--variant rejects invalid value', () => {
  assert.throws(() => parseFlags(['--variant', 'rust']));
});

test('--registry captures value', () => {
  assert.equal(parseFlags(['--registry', 'ghcr.io/foo/']).registry, 'ghcr.io/foo/');
});

test('helpText mentions update and upgrade-cli', () => {
  const text = helpText();
  assert.match(text, /upgrade-cli/);
  assert.match(text, /update/);
});
