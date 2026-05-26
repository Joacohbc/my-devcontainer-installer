import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as path from 'path';
import * as fs from 'fs';
import { resolveWorkspace, projectPaths } from '@/infra/project.js';

test('resolveWorkspace resolves correct workspace name', () => {
  const cwd = '/home/joaco/my-project';
  const resolved = resolveWorkspace(cwd, { workspace: 'custom-workspace' } as any);
  assert.equal(resolved, 'custom-workspace');

  const fallback = resolveWorkspace(cwd, null);
  assert.equal(fallback, 'my-project');
});

test('projectPaths returns correct directory structure', () => {
  const cwd = '/home/joaco/my-project';
  const paths = projectPaths(cwd, 'my-workspace');

  assert.equal(paths.projectDir, path.join(cwd, '.dc_my-workspace'));
  assert.equal(paths.buildDir, path.join(cwd, '.dc_my-workspace', 'build'));
  assert.equal(paths.composeFile, path.join(cwd, '.dc_my-workspace', 'build', 'docker-compose.yml'));
  assert.equal(paths.dockerfilePath, path.join(cwd, '.dc_my-workspace', 'build', 'Dockerfile'));
  assert.equal(paths.envPath, path.join(cwd, '.dc_my-workspace', 'build', '.env'));
});
