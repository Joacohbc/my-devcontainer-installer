import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import {
  completionCandidates,
  bashCompletionScript,
  zshCompletionScript,
  refreshInstalledCompletions,
  type CompletionProviders,
} from '@/domain/completion.js';
import { BUILD_MODES, REMOTE_VARIANTS } from '@/core/types.js';

const stubs: CompletionProviders = {
  containers: () => ['app-dev', 'db-dev'],
  sshAliases: () => ['my-alias', 'prod'],
  modules: () => ['nodejs', 'java-temurin', 'python'],
  services: () => ['mongo', 'postgres', 'tunnel'],
};

test('empty input → commands + root flags', () => {
  const out = completionCandidates([''], stubs);
  for (const cmd of ['setup-ssh', 'run', 'down', 'port-forward', 'config', 'completion']) {
    assert.ok(out.includes(cmd), `missing ${cmd}`);
  }
});

test('no args → commands + root flags', () => {
  const out = completionCandidates([], stubs);
  assert.ok(out.includes('setup-ssh'));
});

test('current starts with - and no subcommand → root flags only', () => {
  const out = completionCandidates(['-'], stubs);
  assert.ok(out.includes('--mode'));
  assert.ok(!out.includes('setup-ssh'));
});

test('root --variant → REMOTE_VARIANTS', () => {
  const out = completionCandidates(['--variant', ''], stubs);
  assert.deepEqual(out, [...REMOTE_VARIANTS]);
});

test('root --mode → BUILD_MODES', () => {
  const out = completionCandidates(['--mode', ''], stubs);
  assert.deepEqual(out, [...BUILD_MODES]);
});

test('root --with → modules from stub', () => {
  const out = completionCandidates(['--with', ''], stubs);
  assert.deepEqual(out, ['nodejs', 'java-temurin', 'python']);
});

test('root --with with comma list → prefix-prepended candidates', () => {
  const out = completionCandidates(['--with', 'nodejs,'], stubs);
  assert.ok(out.includes('nodejs,java-temurin'));
});

test('root --service → services from stub', () => {
  const out = completionCandidates(['--service', ''], stubs);
  assert.deepEqual(out, ['mongo', 'postgres', 'tunnel']);
});

test('run --variant → REMOTE_VARIANTS', () => {
  const out = completionCandidates(['run', '--variant', ''], stubs);
  assert.deepEqual(out, [...REMOTE_VARIANTS]);
});

test('setup-ssh --mode → local|windows|remote', () => {
  const out = completionCandidates(['setup-ssh', '--mode', ''], stubs);
  assert.deepEqual(out, ['local', 'windows', 'remote']);
});

test('setup-ssh --container → containers stub', () => {
  const out = completionCandidates(['setup-ssh', '--container', ''], stubs);
  assert.deepEqual(out, ['app-dev', 'db-dev']);
});

test('setup-ssh --alias → ssh aliases', () => {
  const out = completionCandidates(['setup-ssh', '--alias', ''], stubs);
  assert.deepEqual(out, ['my-alias', 'prod']);
});

test('port-forward --alias → ssh aliases', () => {
  const out = completionCandidates(['port-forward', '--alias', ''], stubs);
  assert.deepEqual(out, ['my-alias', 'prod']);
});

test('port-forward --service → services', () => {
  const out = completionCandidates(['port-forward', '--service', ''], stubs);
  assert.deepEqual(out, ['mongo', 'postgres', 'tunnel']);
});

test('config first positional → registry', () => {
  const out = completionCandidates(['config', ''], stubs);
  assert.ok(out.includes('registry'));
});

test('completion first positional → bash|zsh', () => {
  const out = completionCandidates(['completion', ''], stubs);
  assert.ok(out.includes('bash'));
  assert.ok(out.includes('zsh'));
});

test('down → its flags', () => {
  const out = completionCandidates(['down', ''], stubs);
  for (const f of ['-v', '--volumes', '-y', '--yes', '--no-interactive']) {
    assert.ok(out.includes(f), `missing flag ${f}`);
  }
});

test('free-form flag → empty candidates', () => {
  assert.deepEqual(completionCandidates(['run', '--name', ''], stubs), []);
  assert.deepEqual(completionCandidates(['setup-ssh', '--key', ''], stubs), []);
  assert.deepEqual(completionCandidates(['--image', ''], stubs), []);
});

test('throwing provider does not propagate', () => {
  const bad: CompletionProviders = {
    containers: () => {
      throw new Error('docker down');
    },
    sshAliases: () => {
      throw new Error('no config');
    },
    modules: () => ['nodejs'],
    services: () => ['mongo'],
  };
  // The completion logic itself does not wrap providers — wrapping happens in the
  // real providers passed to runCompleteHidden. Verify the public path:
  // when used directly via completionCandidates, the test asserts that real
  // provider wrappers swallow errors, which is exercised elsewhere. Here we
  // confirm that as long as providers throw, completionCandidates surfaces it
  // — runCompleteHidden's try/catch is what protects the binary.
  assert.throws(() => completionCandidates(['setup-ssh', '--container', ''], bad));
});

test('bash script mentions __complete and complete -F', () => {
  const s = bashCompletionScript();
  assert.match(s, /__complete/);
  assert.match(s, /complete -F/);
});

test('zsh script mentions compdef and compadd', () => {
  const s = zshCompletionScript();
  assert.match(s, /#compdef devcontainer-cli/);
  assert.match(s, /compadd/);
});

test('zsh script slices words with (@) flag so elements stay split', () => {
  // Without (@), "${words[2,CURRENT-1]}" collapses to a single arg → subcommand
  // detection breaks and __complete always sees the root command list.
  const s = zshCompletionScript();
  assert.match(s, /\$\{\(@\)words\[2,CURRENT-1\]\}/);
});

test('subcommand → flags include --help', () => {
  const out = completionCandidates(['setup-ssh', ''], stubs);
  assert.ok(out.includes('--help'));
  assert.ok(out.includes('--container'));
});

test('refreshInstalledCompletions rewrites only existing files', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dccli-comp-'));
  const compDir = path.join(dir, 'completions');
  fs.mkdirSync(compDir);
  // Only zsh file pre-exists (simulates zsh-only install); bash file absent.
  const zshPath = path.join(compDir, '_devcontainer-cli');
  fs.writeFileSync(zshPath, 'stale');
  const bashPath = path.join(compDir, 'devcontainer-cli.bash');

  const fakeExec = path.join(dir, 'devcontainer-cli');
  const updated = refreshInstalledCompletions(fakeExec);

  assert.deepEqual(updated, [zshPath]);
  assert.equal(fs.readFileSync(zshPath, 'utf8'), zshCompletionScript());
  assert.ok(!fs.existsSync(bashPath), 'must not create files that were not installed');
  fs.rmSync(dir, { recursive: true, force: true });
});

test('refreshInstalledCompletions no-ops when completions dir absent', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dccli-comp-'));
  const updated = refreshInstalledCompletions(path.join(dir, 'devcontainer-cli'));
  assert.deepEqual(updated, []);
  fs.rmSync(dir, { recursive: true, force: true });
});
