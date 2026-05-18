import { execSync, spawnSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

const isWin = process.platform === 'win32';
const isMac = process.platform === 'darwin';
const binaryName = isWin ? 'devcontainer-cli.exe' : 'devcontainer-cli';
const distDir = path.resolve('dist');
const binaryPath = path.join(distDir, binaryName);
const blobPath = path.join(distDir, 'sea-prep.blob');

function run(cmd, args, opts = {}) {
  console.log(`==> ${cmd} ${args.join(' ')}`);
  const r = spawnSync(cmd, args, { stdio: 'inherit', shell: isWin, ...opts });
  if (r.status !== 0) {
    throw new Error(`${cmd} exited ${r.status}`);
  }
}

fs.mkdirSync(distDir, { recursive: true });

console.log('==> Bundling CJS for SEA');
run(process.execPath, ['build-sea.mjs']);

console.log('==> Generating SEA blob');
run(process.execPath, ['--experimental-sea-config', 'sea-config.json']);

console.log(`==> Copying node binary -> ${binaryPath}`);
fs.copyFileSync(process.execPath, binaryPath);
if (!isWin) fs.chmodSync(binaryPath, 0o755);

if (isMac) {
  console.log('==> Removing macOS signature');
  run('codesign', ['--remove-signature', binaryPath]);
}

console.log('==> Injecting blob with postject');
const postjectArgs = [
  '-y',
  'postject',
  binaryPath,
  'NODE_SEA_BLOB',
  blobPath,
  '--sentinel-fuse',
  'NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2',
];
if (isMac) postjectArgs.push('--macho-segment-name', 'NODE_SEA');
run('npx', postjectArgs);

if (isMac) {
  console.log('==> Re-signing macOS binary (ad-hoc)');
  run('codesign', ['--sign', '-', binaryPath]);
}

const target = `${isMac ? 'darwin' : isWin ? 'windows' : 'linux'}-${os.arch() === 'arm64' ? 'arm64' : 'x64'}`;
console.log(`\n✔ Built ${binaryPath} (target: ${target})`);
console.log(`  Assets dir must sit next to binary: ${path.resolve('assets')}`);
console.log(`  Test: ${isWin ? binaryPath : './' + path.relative('.', binaryPath)} --help`);
