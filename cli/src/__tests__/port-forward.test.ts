import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import {
  parsePortForwardFlags,
  parsePortMapping,
  parsePortPair,
  parsePortsList,
  parseSshConfigContent,
  getSshAliases,
  portForwardHelp,
} from '@/port-forward.js';

test('parsePortForwardFlags: defaults', () => {
  const f = parsePortForwardFlags([]);
  assert.equal(f.help, false);
  assert.equal(f.interactive, true);
  assert.equal(f.alias, undefined);
  assert.equal(f.service, undefined);
  assert.equal(f.portMapping, undefined);
});

test('parsePortForwardFlags: custom alias and service', () => {
  const f = parsePortForwardFlags(['--alias', 'my-alias', '--service', 'postgres']);
  assert.equal(f.alias, 'my-alias');
  assert.equal(f.service, 'postgres');
});

test('parsePortForwardFlags: interactive flags', () => {
  const f1 = parsePortForwardFlags(['--no-interactive']);
  assert.equal(f1.interactive, false);

  const f2 = parsePortForwardFlags(['--non-interactive']);
  assert.equal(f2.interactive, false);
});

test('parsePortForwardFlags: positional port mapping', () => {
  const f = parsePortForwardFlags(['8080:80']);
  assert.equal(f.portMapping, '8080:80');
});

test('parsePortForwardFlags: help', () => {
  const f1 = parsePortForwardFlags(['-h']);
  assert.equal(f1.help, true);

  const f2 = parsePortForwardFlags(['--help']);
  assert.equal(f2.help, true);
});

test('parsePortForwardFlags: unknown flags throw', () => {
  assert.throws(() => parsePortForwardFlags(['--unknown']));
});

test('parsePortForwardFlags: duplicate port mappings throw', () => {
  assert.throws(() => parsePortForwardFlags(['3000', '8080:80']));
});

test('parsePortMapping: single port', () => {
  const m = parsePortMapping('3000');
  assert.equal(m.localPort, 3000);
  assert.equal(m.targetHost, 'localhost');
  assert.equal(m.containerPort, 3000);
});

test('parsePortMapping: single port with service override', () => {
  const m = parsePortMapping('3000', 'postgres');
  assert.equal(m.localPort, 3000);
  assert.equal(m.targetHost, 'postgres');
  assert.equal(m.containerPort, 3000);
});

test('parsePortMapping: local:container port mapping', () => {
  const m = parsePortMapping('8080:80');
  assert.equal(m.localPort, 8080);
  assert.equal(m.targetHost, 'localhost');
  assert.equal(m.containerPort, 80);
});

test('parsePortMapping: local:container port mapping with service override', () => {
  const m = parsePortMapping('8080:80', 'web');
  assert.equal(m.localPort, 8080);
  assert.equal(m.targetHost, 'web');
  assert.equal(m.containerPort, 80);
});

test('parsePortMapping: triple format', () => {
  const m = parsePortMapping('5432:postgres:5432');
  assert.equal(m.localPort, 5432);
  assert.equal(m.targetHost, 'postgres');
  assert.equal(m.containerPort, 5432);
});

test('parsePortMapping: triple format with matching service', () => {
  const m = parsePortMapping('5432:postgres:5432', 'postgres');
  assert.equal(m.localPort, 5432);
  assert.equal(m.targetHost, 'postgres');
  assert.equal(m.containerPort, 5432);
});

test('parsePortMapping: triple format with conflicting service throws', () => {
  assert.throws(() => parsePortMapping('5432:postgres:5432', 'redis'), /Conflicting target hosts/);
});

test('parsePortMapping: invalid ports', () => {
  assert.throws(() => parsePortMapping('-10'));
  assert.throws(() => parsePortMapping('70000'));
  assert.throws(() => parsePortMapping('abc'));
  assert.throws(() => parsePortMapping('8080:abc'));
  assert.throws(() => parsePortMapping('8080:postgres:abc'));
  assert.throws(() => parsePortMapping('abc:postgres:5432'));
  assert.throws(() => parsePortMapping('5432::5432'));
  assert.throws(() => parsePortMapping('5432:postgres:5432:extra'));
});

test('parsePortPair: single port', () => {
  assert.deepEqual(parsePortPair('3000'), { localPort: 3000, containerPort: 3000 });
});

test('parsePortPair: local:container', () => {
  assert.deepEqual(parsePortPair('8080:80'), { localPort: 8080, containerPort: 80 });
});

test('parsePortPair: trims whitespace', () => {
  assert.deepEqual(parsePortPair(' 8080 : 80 '), { localPort: 8080, containerPort: 80 });
});

test('parsePortPair: invalid', () => {
  assert.throws(() => parsePortPair('abc'));
  assert.throws(() => parsePortPair('0'));
  assert.throws(() => parsePortPair('70000'));
  assert.throws(() => parsePortPair('8080:abc'));
  assert.throws(() => parsePortPair('5432:postgres:5432'), /Invalid port mapping/);
});

test('parsePortsList: comma-separated list', () => {
  assert.deepEqual(parsePortsList('3000, 8080:80 , 5432'), [
    { localPort: 3000, containerPort: 3000 },
    { localPort: 8080, containerPort: 80 },
    { localPort: 5432, containerPort: 5432 },
  ]);
});

test('parsePortsList: single entry', () => {
  assert.deepEqual(parsePortsList('3000'), [{ localPort: 3000, containerPort: 3000 }]);
});

test('parsePortsList: empty throws', () => {
  assert.throws(() => parsePortsList(''));
  assert.throws(() => parsePortsList('  ,  '));
});

test('parsePortsList: propagates invalid entry', () => {
  assert.throws(() => parsePortsList('3000, abc'));
});

test('parseSshConfigContent: extracts hosts correctly', () => {
  const mockConfig = `
# This is a comment
Host devcontainer
  HostName 127.0.0.1
  User vscode
  Port 2222

host service-db
  HostName 10.0.0.5

Host web-server api-server
  HostName 192.168.1.10

Host *
  SendEnv LANG LC_*

Host wildcard?
  User git
`;
  const hosts = parseSshConfigContent(mockConfig);
  assert.deepEqual(hosts, ['devcontainer', 'service-db', 'web-server', 'api-server']);
});

test('getSshAliases: parses mock file correctly', () => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ssh-test-'));
  const tempFile = path.join(tempDir, 'config');
  try {
    fs.writeFileSync(tempFile, 'Host test-alias\n  HostName 1.2.3.4\n');
    const aliases = getSshAliases(tempFile);
    assert.deepEqual(aliases, ['test-alias']);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('portForwardHelp includes usage text', () => {
  assert.match(portForwardHelp(), /Usage:/);
  assert.match(portForwardHelp(), /devcontainer-cli port-forward/);
});
