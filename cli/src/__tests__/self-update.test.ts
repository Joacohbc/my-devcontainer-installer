import { test } from 'node:test';
import assert from 'node:assert/strict';
import { compareVersions, resolveAssetUrl } from '../self-update.js';

test('compareVersions basic ordering', () => {
  assert.equal(compareVersions('1.2.3', '1.2.4'), -1);
  assert.equal(compareVersions('1.2.4', '1.2.3'), 1);
  assert.equal(compareVersions('1.2.3', '1.2.3'), 0);
  assert.equal(compareVersions('2.0.0', '1.9.9'), 1);
});

test('compareVersions strips v prefix', () => {
  assert.equal(compareVersions('v1.2.3', '1.2.3'), 0);
  assert.equal(compareVersions('v1.2.3', 'v1.2.4'), -1);
});

test('compareVersions handles missing patch parts', () => {
  assert.equal(compareVersions('1.2', '1.2.0'), 0);
  assert.equal(compareVersions('1.2', '1.2.1'), -1);
});

test('resolveAssetUrl finds the binary for a triplet', () => {
  const release = {
    tag_name: 'v1.0.0',
    assets: [
      { name: 'devcontainer-cli-linux-x64', browser_download_url: 'https://x/linux-x64' },
      { name: 'devcontainer-cli-linux-arm64', browser_download_url: 'https://x/linux-arm64' },
      { name: 'devcontainer-cli-darwin-x64', browser_download_url: 'https://x/darwin-x64' },
      { name: 'devcontainer-cli-windows-x64.exe', browser_download_url: 'https://x/windows-x64.exe' },
    ],
  };
  assert.equal(resolveAssetUrl(release, 'linux-x64', ''), 'https://x/linux-x64');
  assert.equal(resolveAssetUrl(release, 'linux-arm64', ''), 'https://x/linux-arm64');
  assert.equal(resolveAssetUrl(release, 'darwin-x64', ''), 'https://x/darwin-x64');
  assert.equal(resolveAssetUrl(release, 'windows-x64', '.exe'), 'https://x/windows-x64.exe');
});

test('resolveAssetUrl throws when triplet is missing', () => {
  const release = {
    tag_name: 'v1.0.0',
    assets: [{ name: 'devcontainer-cli-linux-x64', browser_download_url: 'https://x/linux-x64' }],
  };
  assert.throws(() => resolveAssetUrl(release, 'darwin-arm64', ''), /not found/);
});
