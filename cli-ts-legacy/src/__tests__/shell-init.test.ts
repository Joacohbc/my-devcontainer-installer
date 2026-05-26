import { test } from 'node:test';
import assert from 'node:assert/strict';
import { emitShellInit, RC_FILES } from '@/modules/dockerfile/shell-init.js';

test('emitShellInit: writes the init script and sources it from every rc file', () => {
  const out = emitShellInit('.nodejs_init.sh', ['export FOO=bar']);
  assert.match(out, /printf '%s\\n' 'export FOO=bar' > \/home\/devuser\/\.nodejs_init\.sh/);
  assert.match(out, /for f in \.zshrc \.bashrc \.profile/);
  assert.match(out, /echo '\. \\\$HOME\/\.nodejs_init\.sh'/);
});

test('emitShellInit: sources from all RC_FILES (zsh default does not read .profile alone)', () => {
  assert.deepEqual(RC_FILES, ['.zshrc', '.bashrc', '.profile']);
});

test('emitShellInit: escapes $ and " once for the outer double-quoted shell', () => {
  const out = emitShellInit('.bun_init.sh', ['export PATH="$HOME/.bun:$PATH"']);
  assert.match(out, /'export PATH=\\"\\\$HOME\/\.bun:\\\$PATH\\"'/);
});

test('emitShellInit: rejects single quotes in a line', () => {
  assert.throws(() => emitShellInit('.x_init.sh', ["echo 'nope'"]), /single quotes/);
});
