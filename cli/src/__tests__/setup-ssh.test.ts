import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  parseSetupSshFlags,
  setupSshHelp,
  buildConfigBlock,
  hasAliasBlock,
  stripAliasBlock,
} from '@/setup-ssh.js';

// --- parseSetupSshFlags ---

test('parseSetupSshFlags: defaults', () => {
  const f = parseSetupSshFlags([]);
  assert.equal(f.help, false);
  assert.equal(f.assumeYes, false);
  assert.equal(f.mode, '');
  assert.equal(f.aliasExplicit, false);
  assert.equal(f.containerExplicit, false);
  assert.equal(f.serviceExplicit, false);
  assert.equal(f.composeFileExplicit, false);
});

test('parseSetupSshFlags: --help / -h', () => {
  assert.equal(parseSetupSshFlags(['--help']).help, true);
  assert.equal(parseSetupSshFlags(['-h']).help, true);
});

test('parseSetupSshFlags: --yes / -y', () => {
  assert.equal(parseSetupSshFlags(['--yes']).assumeYes, true);
  assert.equal(parseSetupSshFlags(['-y']).assumeYes, true);
});

test('parseSetupSshFlags: --alias sets aliasExplicit', () => {
  const f = parseSetupSshFlags(['--alias', 'myalias']);
  assert.equal(f.alias, 'myalias');
  assert.equal(f.aliasExplicit, true);
});

test('parseSetupSshFlags: --container sets containerExplicit', () => {
  const f = parseSetupSshFlags(['--container', 'mycontainer']);
  assert.equal(f.container, 'mycontainer');
  assert.equal(f.containerExplicit, true);
});

test('parseSetupSshFlags: --service sets serviceExplicit', () => {
  const f = parseSetupSshFlags(['--service', 'myservice']);
  assert.equal(f.service, 'myservice');
  assert.equal(f.serviceExplicit, true);
});

test('parseSetupSshFlags: --compose-file / -f sets composeFileExplicit', () => {
  const f1 = parseSetupSshFlags(['--compose-file', './my/compose.yml']);
  assert.equal(f1.composeFile, './my/compose.yml');
  assert.equal(f1.composeFileExplicit, true);

  const f2 = parseSetupSshFlags(['-f', './other.yml']);
  assert.equal(f2.composeFile, './other.yml');
  assert.equal(f2.composeFileExplicit, true);
});

test('parseSetupSshFlags: --remote sets mode to remote', () => {
  const f = parseSetupSshFlags(['--remote', 'user@host']);
  assert.equal(f.remote, 'user@host');
  assert.equal(f.mode, 'remote');
});

test('parseSetupSshFlags: --mode valid values', () => {
  assert.equal(parseSetupSshFlags(['--mode', 'local']).mode, 'local');
  assert.equal(parseSetupSshFlags(['--mode', 'windows']).mode, 'windows');
  assert.equal(parseSetupSshFlags(['--mode', 'remote']).mode, 'remote');
});

test('parseSetupSshFlags: --mode invalid throws', () => {
  assert.throws(() => parseSetupSshFlags(['--mode', 'invalid']));
});

test('parseSetupSshFlags: unknown flag throws', () => {
  assert.throws(() => parseSetupSshFlags(['--unknown']));
});

test('parseSetupSshFlags: --key and --port', () => {
  const f = parseSetupSshFlags(['--key', '/home/user/.ssh/id_test', '--port', '2222']);
  assert.equal(f.key, '/home/user/.ssh/id_test');
  assert.equal(f.port, '2222');
});

// --- setupSshHelp ---

test('setupSshHelp: contains usage section', () => {
  const h = setupSshHelp();
  assert.match(h, /Usage:/);
  assert.match(h, /setup-ssh/);
  assert.match(h, /--alias/);
  assert.match(h, /--remote/);
  assert.match(h, /--compose-file/);
});

// --- hasAliasBlock ---

const sampleConfig = `
Host joaco
    HostName 172.25.0.2
    User devuser
    IdentityFile ~/.ssh/id_joaco

Host other
    HostName 10.0.0.1
`;

test('hasAliasBlock: finds existing alias', () => {
  assert.equal(hasAliasBlock(sampleConfig, 'joaco'), true);
  assert.equal(hasAliasBlock(sampleConfig, 'other'), true);
});

test('hasAliasBlock: returns false for missing alias', () => {
  assert.equal(hasAliasBlock(sampleConfig, 'nonexistent'), false);
  assert.equal(hasAliasBlock('', 'joaco'), false);
});

test('hasAliasBlock: does not partial-match', () => {
  assert.equal(hasAliasBlock(sampleConfig, 'joa'), false);
  assert.equal(hasAliasBlock(sampleConfig, 'joaco2'), false);
});

// --- stripAliasBlock ---

test('stripAliasBlock: removes the target host block', () => {
  const result = stripAliasBlock(sampleConfig, 'joaco');
  assert.ok(!result.includes('Host joaco'));
  assert.ok(!result.includes('id_joaco'));
  assert.ok(result.includes('Host other'));
});

test('stripAliasBlock: leaves other blocks intact', () => {
  const result = stripAliasBlock(sampleConfig, 'other');
  assert.ok(result.includes('Host joaco'));
  assert.ok(!result.includes('Host other'));
});

test('stripAliasBlock: no-op when alias not found', () => {
  const result = stripAliasBlock(sampleConfig, 'nothere');
  assert.equal(result, sampleConfig);
});

// --- buildConfigBlock ---

test('buildConfigBlock: local mode', () => {
  const block = buildConfigBlock('local', {
    remote: '',
    alias: 'joaco',
    aliasExplicit: false,
    key: '/home/user/.ssh/id_joaco',
    port: '2222',
    mode: 'local',
    assumeYes: false,
    container: 'joaco-devcontainer-ssh',
    containerExplicit: false,
    service: 'devcontainer-ssh',
    serviceExplicit: false,
    composeFile: '.dc_joaco/build/docker-compose.yml',
    composeFileExplicit: false,
    user: 'devuser',
    help: false,
  }, { hostname: '172.25.0.2', port: '' });

  assert.match(block, /Host joaco/);
  assert.match(block, /HostName 172\.25\.0\.2/);
  assert.match(block, /User devuser/);
  assert.match(block, /IdentityFile .*id_joaco/);
});

test('buildConfigBlock: windows mode', () => {
  const block = buildConfigBlock('windows', {
    remote: '',
    alias: 'joaco',
    aliasExplicit: false,
    key: '/home/user/.ssh/id_joaco',
    port: '2222',
    mode: 'windows',
    assumeYes: false,
    container: 'joaco-devcontainer-ssh',
    containerExplicit: false,
    service: 'devcontainer-ssh',
    serviceExplicit: false,
    composeFile: '.dc_joaco/build/docker-compose.yml',
    composeFileExplicit: false,
    user: 'devuser',
    help: false,
  }, { hostname: 'localhost', port: '2222' });

  assert.match(block, /Host joaco/);
  assert.match(block, /HostName localhost/);
  assert.match(block, /Port 2222/);
});

test('buildConfigBlock: remote mode', () => {
  const block = buildConfigBlock('remote', {
    remote: 'user@myserver',
    alias: 'joaco',
    aliasExplicit: false,
    key: '/home/user/.ssh/id_joaco',
    port: '2222',
    mode: 'remote',
    assumeYes: false,
    container: 'joaco-devcontainer-ssh',
    containerExplicit: false,
    service: 'devcontainer-ssh',
    serviceExplicit: false,
    composeFile: '.dc_joaco/build/docker-compose.yml',
    composeFileExplicit: false,
    user: 'devuser',
    help: false,
  }, { hostname: '', port: '' });

  assert.match(block, /Host joaco/);
  assert.match(block, /ProxyCommand/);
  assert.match(block, /user@myserver/);
  assert.match(block, /joaco-devcontainer-ssh/);
});
