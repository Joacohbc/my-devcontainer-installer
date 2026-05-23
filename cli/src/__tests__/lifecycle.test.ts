import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseLifecycleFlags, lifecycleHelp } from '@/lifecycle.js';

test('parseLifecycleFlags: defaults', () => {
  assert.equal(parseLifecycleFlags('start', []).help, false);
});

test('parseLifecycleFlags: --help', () => {
  assert.equal(parseLifecycleFlags('stop', ['--help']).help, true);
  assert.equal(parseLifecycleFlags('restart', ['-h']).help, true);
});

test('parseLifecycleFlags: unknown flag throws with verb name', () => {
  assert.throws(() => parseLifecycleFlags('start', ['--banana']), /start/);
});

test('lifecycleHelp mentions the verb', () => {
  assert.match(lifecycleHelp('start'), /docker compose start/);
  assert.match(lifecycleHelp('stop'), /docker compose stop/);
  assert.match(lifecycleHelp('restart'), /docker compose restart/);
});
