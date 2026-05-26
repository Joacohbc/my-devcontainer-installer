import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parsePruneFlags, pruneHelp } from '@/commands/prune.js';

test('parsePruneFlags: defaults', () => {
  const f = parsePruneFlags([]);
  assert.equal(f.all, false);
  assert.equal(f.yes, false);
  assert.equal(f.interactive, true);
  assert.equal(f.help, false);
});

test('parsePruneFlags: --all', () => {
  assert.equal(parsePruneFlags(['--all']).all, true);
});

test('parsePruneFlags: --yes / -y', () => {
  assert.equal(parsePruneFlags(['--yes']).yes, true);
  assert.equal(parsePruneFlags(['-y']).yes, true);
});

test('parsePruneFlags: --help', () => {
  assert.equal(parsePruneFlags(['--help']).help, true);
});

test('parsePruneFlags: --no-interactive', () => {
  assert.equal(parsePruneFlags(['--no-interactive']).interactive, false);
});

test('parsePruneFlags: unknown flag throws', () => {
  assert.throws(() => parsePruneFlags(['--banana']));
});

test('pruneHelp mentions --all and orphan', () => {
  const help = pruneHelp();
  assert.match(help, /--all/);
  assert.match(help, /orphan/);
  assert.match(help, /devcontainer-cli/);
});
