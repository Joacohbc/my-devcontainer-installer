import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseQuickRunFlags, quickRunHelp } from '@/commands/quick-run.js';

test('parseQuickRunFlags: --variant', () => {
  assert.equal(parseQuickRunFlags(['--variant', 'python']).variant, 'python');
});

test('parseQuickRunFlags: --name', () => {
  assert.equal(parseQuickRunFlags(['--name', 'mycontainer']).name, 'mycontainer');
});

test('parseQuickRunFlags: --volume', () => {
  assert.equal(parseQuickRunFlags(['--volume', 'my_vol']).volume, 'my_vol');
});

test('parseQuickRunFlags: --port', () => {
  assert.equal(parseQuickRunFlags(['--port', '2222']).port, 2222);
});

test('parseQuickRunFlags: --port rejects invalid', () => {
  assert.throws(() => parseQuickRunFlags(['--port', '99999']));
  assert.throws(() => parseQuickRunFlags(['--port', 'abc']));
});

test('parseQuickRunFlags: --registry', () => {
  assert.equal(parseQuickRunFlags(['--registry', 'ghcr.io/foo/']).registry, 'ghcr.io/foo/');
});

test('parseQuickRunFlags: --variant rejects unknown', () => {
  assert.throws(() => parseQuickRunFlags(['--variant', 'rust']));
});

test('parseQuickRunFlags: unknown flag throws', () => {
  assert.throws(() => parseQuickRunFlags(['--banana']));
});

test('parseQuickRunFlags: --no-interactive', () => {
  assert.equal(parseQuickRunFlags(['--no-interactive']).interactive, false);
});

test('parseQuickRunFlags: --variant survives surrounding common flags', () => {
  const f = parseQuickRunFlags(['--no-interactive', '--variant', 'python']);
  assert.equal(f.variant, 'python');
  assert.equal(f.interactive, false);
});

test('quickRunHelp mentions all variants', () => {
  const help = quickRunHelp();
  assert.match(help, /ssh/);
  assert.match(help, /node-java-temurin/);
  assert.match(help, /python/);
  assert.match(help, /--volume/);
  assert.match(help, /--port/);
});
