import { describe, it, beforeEach, afterEach } from 'node:test';
import * as assert from 'node:assert';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { parsePortForwardFlags, getAvailableSshHosts } from '../port-forward.js';

describe('parsePortForwardFlags', () => {
  it('parses only portMapping', () => {
    const flags = parsePortForwardFlags(['3000']);
    assert.deepStrictEqual(flags, { portMapping: '3000', alias: null, service: null });
  });

  it('parses portMapping and --alias', () => {
    const flags = parsePortForwardFlags(['8080:80', '--alias', 'my-host']);
    assert.deepStrictEqual(flags, { portMapping: '8080:80', alias: 'my-host', service: null });
  });

  it('parses portMapping and --service', () => {
    const flags = parsePortForwardFlags(['5432:5432', '--service', 'postgres']);
    assert.deepStrictEqual(flags, { portMapping: '5432:5432', alias: null, service: 'postgres' });
  });

  it('parses all flags', () => {
    const flags = parsePortForwardFlags(['3000', '--service', 'web', '--alias', 'dev']);
    assert.deepStrictEqual(flags, { portMapping: '3000', alias: 'dev', service: 'web' });
  });

  it('allows flags before portMapping', () => {
    const flags = parsePortForwardFlags(['--alias', 'dev', '3000']);
    assert.deepStrictEqual(flags, { portMapping: '3000', alias: 'dev', service: null });
  });

  it('throws on duplicate --alias', () => {
    assert.throws(
      () => parsePortForwardFlags(['--alias', 'a', '--alias', 'b']),
      /Duplicate flag: --alias/
    );
  });

  it('throws on duplicate --service', () => {
    assert.throws(
      () => parsePortForwardFlags(['--service', 'a', '--service', 'b']),
      /Duplicate flag: --service/
    );
  });

  it('throws on missing value for --alias', () => {
    assert.throws(() => parsePortForwardFlags(['--alias']), /Missing value for --alias/);
  });

  it('throws on missing value for --service', () => {
    assert.throws(() => parsePortForwardFlags(['--service']), /Missing value for --service/);
  });

  it('throws on unknown flags', () => {
    assert.throws(() => parsePortForwardFlags(['--unknown']), /Unknown flag: --unknown/);
  });

  it('throws on too many positional arguments', () => {
    assert.throws(() => parsePortForwardFlags(['3000', '4000']), /Too many positional arguments: 4000/);
  });
});

describe('getAvailableSshHosts', () => {
  let tempDir: string;
  let mockConfigPath: string;

  beforeEach(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ssh-test-'));
    mockConfigPath = path.join(tempDir, 'config');
  });

  afterEach(() => {
    fs.rmSync(tempDir, { recursive: true, force: true });
  });

  it('returns empty array if file does not exist', () => {
    assert.deepStrictEqual(getAvailableSshHosts(mockConfigPath), []);
  });

  it('extracts single host', () => {
    fs.writeFileSync(mockConfigPath, 'Host devcontainer\n  HostName localhost');
    assert.deepStrictEqual(getAvailableSshHosts(mockConfigPath), ['devcontainer']);
  });

  it('extracts multiple hosts', () => {
    const content = `
      Host dev1
        HostName 1.1.1.1

      Host dev2
        HostName 2.2.2.2
    `;
    fs.writeFileSync(mockConfigPath, content);
    const hosts = getAvailableSshHosts(mockConfigPath);
    assert.ok(hosts.includes('dev1'));
    assert.ok(hosts.includes('dev2'));
    assert.strictEqual(hosts.length, 2);
  });

  it('ignores Host *', () => {
    const content = `
      Host *
        IdentityFile ~/.ssh/id_rsa

      Host dev
        HostName localhost
    `;
    fs.writeFileSync(mockConfigPath, content);
    assert.deepStrictEqual(getAvailableSshHosts(mockConfigPath), ['dev']);
  });

  it('handles inline comments and spaces', () => {
    const content = `
      Host   dev1  # This is dev1
      Host dev2 dev3
      Host\tdev4
    `;
    fs.writeFileSync(mockConfigPath, content);
    const hosts = getAvailableSshHosts(mockConfigPath);
    assert.ok(hosts.includes('dev1'));
    assert.ok(hosts.includes('dev2'));
    assert.ok(hosts.includes('dev3'));
    assert.ok(hosts.includes('dev4'));
    assert.strictEqual(hosts.length, 4);
  });
});
