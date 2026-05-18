import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as https from 'https';
import { URL } from 'url';
import chalk from 'chalk';

const REPO = 'Joacohbc/my-devcontainer-installer';
const USER_AGENT = 'devcontainer-cli';

interface ReleaseAsset {
  name: string;
  browser_download_url: string;
}

interface Release {
  tag_name: string;
  assets: ReleaseAsset[];
}

export function getCurrentVersion(): string {
  try {
    return __CLI_VERSION__;
  } catch {
    return 'dev';
  }
}

export function getTargetTriplet(): { triplet: string; ext: string } {
  const arch = os.arch();
  const platform = process.platform;
  let osName: string;
  let archName: string;

  if (platform === 'linux') osName = 'linux';
  else if (platform === 'darwin') osName = 'darwin';
  else if (platform === 'win32') osName = 'windows';
  else throw new Error(`unsupported platform: ${platform}`);

  if (arch === 'arm64') {
    // Apple Silicon: match install.sh and use darwin-x64 (via Rosetta).
    archName = osName === 'darwin' ? 'x64' : 'arm64';
  } else if (arch === 'x64') {
    archName = 'x64';
  } else {
    throw new Error(`unsupported arch: ${arch}`);
  }

  return {
    triplet: `${osName}-${archName}`,
    ext: osName === 'windows' ? '.exe' : '',
  };
}

export function compareVersions(a: string, b: string): number {
  const normalize = (s: string) => s.replace(/^v/, '').split(/[.\-+]/);
  const pa = normalize(a);
  const pb = normalize(b);
  const len = Math.max(pa.length, pb.length);
  for (let i = 0; i < len; i++) {
    const ai = pa[i] ?? '0';
    const bi = pb[i] ?? '0';
    const an = Number(ai);
    const bn = Number(bi);
    const bothNumeric = !Number.isNaN(an) && !Number.isNaN(bn);
    if (bothNumeric) {
      if (an !== bn) return an < bn ? -1 : 1;
    } else {
      if (ai !== bi) return ai < bi ? -1 : 1;
    }
  }
  return 0;
}

export function resolveAssetUrl(release: Release, triplet: string, ext: string): string {
  const name = `devcontainer-cli-${triplet}${ext}`;
  const asset = release.assets.find((a) => a.name === name);
  if (!asset) {
    const available = release.assets.map((a) => a.name).join(', ');
    throw new Error(`asset ${name} not found in release ${release.tag_name}. Available: ${available || '(none)'}`);
  }
  return asset.browser_download_url;
}

function httpGetJson(url: string, redirectsLeft = 5): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const headers: Record<string, string> = {
      'User-Agent': USER_AGENT,
      Accept: 'application/vnd.github+json',
    };
    if (process.env.GITHUB_TOKEN) {
      headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
    }
    https
      .get(url, { headers }, (res) => {
        const status = res.statusCode ?? 0;
        if (status >= 300 && status < 400 && res.headers.location && redirectsLeft > 0) {
          res.resume();
          const next = new URL(res.headers.location, url).toString();
          httpGetJson(next, redirectsLeft - 1).then(resolve, reject);
          return;
        }
        const chunks: Buffer[] = [];
        res.on('data', (c: Buffer) => chunks.push(c));
        res.on('end', () => resolve({ status, body: Buffer.concat(chunks).toString('utf8') }));
        res.on('error', reject);
      })
      .on('error', reject);
  });
}

export async function fetchLatestRelease(repo: string = REPO): Promise<Release> {
  const url = `https://api.github.com/repos/${repo}/releases/latest`;
  const { status, body } = await httpGetJson(url);
  if (status === 403) {
    throw new Error('GitHub API rate-limited (403). Set GITHUB_TOKEN to authenticate.');
  }
  if (status !== 200) {
    throw new Error(`GitHub API ${status}: ${body.slice(0, 200)}`);
  }
  const parsed = JSON.parse(body) as Release;
  if (!parsed.tag_name || !Array.isArray(parsed.assets)) {
    throw new Error('unexpected GitHub release payload');
  }
  return parsed;
}

function downloadBinary(url: string, destTmp: string, redirectsLeft = 5): Promise<void> {
  return new Promise((resolve, reject) => {
    const req = https.get(url, { headers: { 'User-Agent': USER_AGENT } }, (res) => {
      const status = res.statusCode ?? 0;
      if (status >= 300 && status < 400 && res.headers.location && redirectsLeft > 0) {
        res.resume();
        const next = new URL(res.headers.location, url).toString();
        downloadBinary(next, destTmp, redirectsLeft - 1).then(resolve, reject);
        return;
      }
      if (status !== 200) {
        res.resume();
        reject(new Error(`download failed: HTTP ${status} ${url}`));
        return;
      }
      const out = fs.createWriteStream(destTmp);
      res.pipe(out);
      out.on('finish', () => out.close(() => resolve()));
      out.on('error', reject);
      res.on('error', reject);
    });
    req.on('error', reject);
  });
}

function replaceBinary(tmpPath: string): void {
  const execPath = process.execPath;
  if (process.platform === 'win32') {
    const oldPath = execPath + '.old';
    try {
      fs.unlinkSync(oldPath);
    } catch {
      // best-effort
    }
    fs.renameSync(execPath, oldPath);
    fs.renameSync(tmpPath, execPath);
  } else {
    fs.chmodSync(tmpPath, 0o755);
    // rename is atomic and works on POSIX even when execPath is currently executing.
    fs.renameSync(tmpPath, execPath);
  }
}

export function cleanupStaleUpdate(): void {
  if (process.platform !== 'win32') return;
  const oldPath = process.execPath + '.old';
  try {
    fs.unlinkSync(oldPath);
  } catch {
    // either doesn't exist or is locked — try again next run
  }
}

interface SelfUpdateFlags {
  check: boolean;
  force: boolean;
  help: boolean;
}

function parseSelfUpdateFlags(argv: string[]): SelfUpdateFlags {
  const flags: SelfUpdateFlags = { check: false, force: false, help: false };
  for (const a of argv) {
    switch (a) {
      case '--check': flags.check = true; break;
      case '--force': flags.force = true; break;
      case '-h':
      case '--help': flags.help = true; break;
      default: throw new Error(`Unknown flag for update: ${a}`);
    }
  }
  return flags;
}

function helpText(): string {
  return `devcontainer-cli update — replace the current binary with the latest release

Usage:
  devcontainer-cli update [--check] [--force]

Flags:
  --check    Print current vs latest and exit (no download)
  --force    Reinstall even if the current version matches latest
  -h, --help Show this help

Env:
  GITHUB_TOKEN  Optional, to avoid the 60 req/hour anonymous rate limit
`;
}

export async function runSelfUpdate(argv: string[]): Promise<void> {
  const flags = parseSelfUpdateFlags(argv);
  if (flags.help) {
    console.log(helpText());
    return;
  }

  const current = getCurrentVersion();
  console.log(chalk.gray(`Current version: ${current}`));

  console.log(chalk.gray('Fetching latest release...'));
  const release = await fetchLatestRelease();
  const latest = release.tag_name;
  console.log(chalk.gray(`Latest version : ${latest}`));

  const cmp = current === 'dev' ? -1 : compareVersions(current, latest);
  if (flags.check) {
    if (cmp < 0) console.log(chalk.yellow(`Update available: ${current} → ${latest}`));
    else if (cmp === 0) console.log(chalk.green('Already up to date.'));
    else console.log(chalk.gray('Current version is ahead of latest release.'));
    return;
  }

  if (cmp >= 0 && !flags.force) {
    console.log(chalk.green('Already up to date.'));
    return;
  }

  const { triplet, ext } = getTargetTriplet();
  const url = resolveAssetUrl(release, triplet, ext);
  console.log(chalk.gray(`Downloading ${url}`));

  const tmpPath = path.join(path.dirname(process.execPath), `.devcontainer-cli.${process.pid}.download`);
  try {
    await downloadBinary(url, tmpPath);
    const stat = fs.statSync(tmpPath);
    if (stat.size === 0) throw new Error('downloaded file is empty');
    replaceBinary(tmpPath);
  } catch (e) {
    try { fs.unlinkSync(tmpPath); } catch { /* noop */ }
    throw e;
  }

  console.log(chalk.green(`✅ Updated to ${latest}. Restart any running session to use the new binary.`));
}
