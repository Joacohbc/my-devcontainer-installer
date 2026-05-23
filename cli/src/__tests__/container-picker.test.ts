import { test } from 'node:test';
import assert from 'node:assert/strict';
import { containerWorkspace, listManagedContainers } from '../container-picker.js';

test('containerWorkspace: strips devcontainer-ssh suffix', () => {
  assert.equal(containerWorkspace('joaco-devcontainer-ssh'), 'joaco');
  assert.equal(containerWorkspace('my-project-devcontainer-ssh'), 'my-project');
  assert.equal(containerWorkspace('workspace-name-devcontainer-ssh'), 'workspace-name');
});

test('containerWorkspace: returns null when suffix not present', () => {
  assert.equal(containerWorkspace('joaco'), null);
  assert.equal(containerWorkspace('joaco-devcontainer'), null);
  assert.equal(containerWorkspace('devcontainer-ssh'), null);
  assert.equal(containerWorkspace(''), null);
});

test('containerWorkspace: does not partially match', () => {
  assert.equal(containerWorkspace('devcontainer-ssh-extra'), null);
  assert.equal(containerWorkspace('joaco-devcontainer-ssh-extra'), null);
});

test('listManagedContainers: returns empty array when docker unavailable or no containers', () => {
  // This test validates the function handles failures gracefully.
  // In a CI env without docker the result is always [].
  const result = listManagedContainers();
  assert.ok(Array.isArray(result));
  for (const c of result) {
    assert.ok(typeof c.name === 'string');
    assert.ok(typeof c.image === 'string');
    assert.ok(typeof c.status === 'string');
    assert.ok(typeof c.state === 'string');
  }
});
