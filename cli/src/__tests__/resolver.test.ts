import { test } from 'node:test';
import assert from 'node:assert/strict';
import { resolveDockerfileModules, ResolverError } from '@/domain/resolver.js';

test('always-on modules are always included', () => {
  const r = resolveDockerfileModules([]);
  const ids = r.map((x) => x.module.id);
  assert.ok(ids.includes('base'));
  assert.ok(ids.includes('cleanup'));
});

test('requires are auto-added', () => {
  const r = resolveDockerfileModules([{ id: 'nodejs' }]);
  const ids = r.map((x) => x.module.id);
  assert.ok(ids.includes('github-cli'), 'github-cli should be auto-added (nodejs requires it)');
  assert.ok(ids.includes('nodejs'));
});

test('unknown module throws', () => {
  assert.throws(() => resolveDockerfileModules([{ id: 'nonexistent' }]), ResolverError);
});

test('category ordering: base first, cleanup last', () => {
  const r = resolveDockerfileModules([{ id: 'java-temurin' }, { id: 'dod' }, { id: 'nodejs' }]);
  const ids = r.map((x) => x.module.id);
  assert.equal(ids[0], 'base');
  assert.equal(ids[ids.length - 1], 'cleanup');
});

test('options pass through', () => {
  const r = resolveDockerfileModules([{ id: 'java-temurin', options: { versions: ['17'] } }]);
  const java = r.find((x) => x.module.id === 'java-temurin')!;
  assert.deepEqual(java.options.versions, ['17']);
});

test('java-temurin conflicts with java-openjdk', () => {
  assert.throws(
    () => resolveDockerfileModules([{ id: 'java-temurin' }, { id: 'java-openjdk' }]),
    ResolverError,
  );
});
