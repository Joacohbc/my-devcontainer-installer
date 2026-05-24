import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as https from 'https';
import * as crypto from 'crypto';
import { URL } from 'url';
import chalk from 'chalk';
import { refreshInstalledCompletions } from '@/completion.js';

const REPO = 'Joacohbc/my-devcontainer-installer';
const USER_AGENT = 'devcontainer-cli';
const PRIMARY_API_HOST = 'api.github.com';
const MAX_DOWNLOAD_BYTES = 200 * 1024 * 1024;
const MAX_CHECKSUM_BYTES = 1024;

const ALLOWED_HOST_SUFFIXES = [
  '.github.com',
  '.githubusercontent.com',
];
const ALLOWED_HOSTS_EXACT = new Set([
  'github.com',
  'api.github.com',
]);

interface ReleaseAsset {
  name: string;
  browser_download_url: string;
}

interface Release {
  tag_name: string;
  assets: ReleaseAsset[];
}

interface AssetUrls {
  binary: string;
  checksum: string;
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

function splitVersion(s: string): { main: string[]; pre: string | null } {
  const noV = s.replace(/^v/, '');
  const dashIdx = noV.indexOf('-');
  if (dashIdx === -1) return { main: noV.split('.'), pre: null };
  return { main: noV.slice(0, dashIdx).split('.'), pre: noV.slice(dashIdx + 1) };
}

export function compareVersions(a: string, b: string): number {
  const va = splitVersion(a);
  const vb = splitVersion(b);
  const len = Math.max(va.main.length, vb.main.length);
  for (let i = 0; i < len; i++) {
    const ai = va.main[i] ?? '0';
    const bi = vb.main[i] ?? '0';
    const an = Number(ai);
    const bn = Number(bi);
    const bothNumeric = !Number.isNaN(an) && !Number.isNaN(bn);
    if (bothNumeric) {
      if (an !== bn) return an < bn ? -1 : 1;
    } else if (ai !== bi) {
      return ai < bi ? -1 : 1;
    }
  }
  if (va.pre === null && vb.pre === null) return 0;
  if (va.pre === null) return 1;
  if (vb.pre === null) return -1;
  if (va.pre === vb.pre) return 0;
  return va.pre < vb.pre ? -1 : 1;
}

export function resolveAssetUrl(release: Release, triplet: string, ext: string): AssetUrls {
  const name = `devcontainer-cli-${triplet}${ext}`;
  const asset = release.assets.find((a) => a.name === name);
  if (!asset) {
    const available = release.assets.map((a) => a.name).join(', ');
    throw new Error(`asset ${name} not found in release ${release.tag_name}. Available: ${available || '(none)'}`);
  }
  const checksumName = `${name}.sha256`;
  const checksumAsset = release.assets.find((a) => a.name === checksumName);
  if (!checksumAsset) {
    throw new Error(`checksum ${checksumName} not found in release ${release.tag_name}. Refusing to update without integrity verification.`);
  }
  return { binary: asset.browser_download_url, checksum: checksumAsset.browser_download_url };
}

function isAllowedHost(host: string): boolean {
  if (ALLOWED_HOSTS_EXACT.has(host)) return true;
  return ALLOWED_HOST_SUFFIXES.some((s) => host.endsWith(s));
}

function assertHttps(u: URL): void {
  if (u.protocol !== 'https:') {
    throw new Error(`refusing non-https URL: ${u.toString()}`);
  }
}

interface HttpOpts {
  withAuth: boolean;
  redirectsLeft?: number;
  maxBytes: number;
}

function httpGetBuffer(url: string, opts: HttpOpts): Promise<{ status: number; body: Buffer }> {
  const redirectsLeft = opts.redirectsLeft ?? 5;
  return new Promise((resolve, reject) => {
    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch (e) {
      reject(e);
      return;
    }
    try {
      assertHttps(parsed);
      if (!isAllowedHost(parsed.host)) {
        throw new Error(`host not in allowlist: ${parsed.host}`);
      }
    } catch (e) {
      reject(e);
      return;
    }

    const headers: Record<string, string> = {
      'User-Agent': USER_AGENT,
      Accept: 'application/vnd.github+json',
    };
    if (opts.withAuth && process.env.GITHUB_TOKEN) {
      headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
    }

    https
      .get(url, { headers }, (res) => {
        const status = res.statusCode ?? 0;
        if (status >= 300 && status < 400 && res.headers.location && redirectsLeft > 0) {
          res.resume();
          let next: URL;
          try {
            next = new URL(res.headers.location, url);
            assertHttps(next);
            if (!isAllowedHost(next.host)) {
              reject(new Error(`redirect to disallowed host: ${next.host}`));
              return;
            }
          } catch (e) {
            reject(e as Error);
            return;
          }
          const sameHost = next.host === parsed.host;
          httpGetBuffer(next.toString(), {
            withAuth: opts.withAuth && sameHost && parsed.host === PRIMARY_API_HOST,
            redirectsLeft: redirectsLeft - 1,
            maxBytes: opts.maxBytes,
          }).then(resolve, reject);
          return;
        }
        const chunks: Buffer[] = [];
        let total = 0;
        res.on('data', (c: Buffer) => {
          total += c.length;
          if (total > opts.maxBytes) {
            res.destroy(new Error(`response exceeded ${opts.maxBytes} bytes`));
            return;
          }
          chunks.push(c);
        });
        res.on('end', () => {
          if (total > opts.maxBytes) return;
          resolve({ status, body: Buffer.concat(chunks) });
        });
        res.on('error', reject);
      })
      .on('error', reject);
  });
}

export async function fetchLatestRelease(repo: string = REPO): Promise<Release> {
  const url = `https://api.github.com/repos/${repo}/releases/latest`;
  const { status, body } = await httpGetBuffer(url, { withAuth: true, maxBytes: 5 * 1024 * 1024 });
  const text = body.toString('utf8');
  if (status === 403) {
    throw new Error('GitHub API rate-limited (403). Set GITHUB_TOKEN to authenticate.');
  }
  if (status !== 200) {
    throw new Error(`GitHub API ${status}: ${text.slice(0, 200)}`);
  }
  let parsed: Release;
  try {
    parsed = JSON.parse(text) as Release;
  } catch (e) {
    throw new Error(`failed to parse GitHub release JSON: ${(e as Error).message}`);
  }
  if (!parsed.tag_name || !Array.isArray(parsed.assets)) {
    throw new Error('unexpected GitHub release payload');
  }
  return parsed;
}

function downloadToFile(url: string, destTmp: string, redirectsLeft = 5): Promise<void> {
  return new Promise((resolve, reject) => {
    let parsed: URL;
    try {
      parsed = new URL(url);
      assertHttps(parsed);
      if (!isAllowedHost(parsed.host)) {
        throw new Error(`host not in allowlist: ${parsed.host}`);
      }
    } catch (e) {
      reject(e as Error);
      return;
    }

    const req = https.get(url, { headers: { 'User-Agent': USER_AGENT } }, (res) => {
      const status = res.statusCode ?? 0;
      if (status >= 300 && status < 400 && res.headers.location && redirectsLeft > 0) {
        res.resume();
        let next: URL;
        try {
          next = new URL(res.headers.location, url);
          assertHttps(next);
          if (!isAllowedHost(next.host)) {
            reject(new Error(`redirect to disallowed host: ${next.host}`));
            return;
          }
        } catch (e) {
          reject(e as Error);
          return;
        }
        downloadToFile(next.toString(), destTmp, redirectsLeft - 1).then(resolve, reject);
        return;
      }
      if (status !== 200) {
        res.resume();
        reject(new Error(`download failed: HTTP ${status} ${url}`));
        return;
      }
      const out = fs.createWriteStream(destTmp, { flags: 'wx', mode: 0o600 });
      let bytes = 0;
      let aborted = false;
      const abort = (err: Error) => {
        if (aborted) return;
        aborted = true;
        res.destroy();
        out.destroy();
        reject(err);
      };
      res.on('data', (chunk: Buffer) => {
        bytes += chunk.length;
        if (bytes > MAX_DOWNLOAD_BYTES) {
          abort(new Error(`download exceeded ${MAX_DOWNLOAD_BYTES} bytes`));
        }
      });
      res.pipe(out);
      out.on('finish', () => {
        if (aborted) return;
        out.close(() => resolve());
      });
      out.on('error', abort);
      res.on('error', abort);
    });
    req.on('error', reject);
  });
}

function sha256File(p: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    const stream = fs.createReadStream(p);
    stream.on('data', (d) => hash.update(d as Buffer));
    stream.on('end', () => resolve(hash.digest('hex')));
    stream.on('error', reject);
  });
}

function parseChecksum(body: Buffer): string {
  const text = body.toString('utf8').trim();
  const first = text.split(/\s+/)[0] ?? '';
  if (!/^[a-fA-F0-9]{64}$/.test(first)) {
    throw new Error(`invalid sha256 checksum content: ${text.slice(0, 80)}`);
  }
  return first.toLowerCase();
}

export async function verifyChecksum(filePath: string, checksumUrl: string): Promise<void> {
  const { status, body } = await httpGetBuffer(checksumUrl, { withAuth: false, maxBytes: MAX_CHECKSUM_BYTES });
  if (status !== 200) {
    throw new Error(`failed to fetch checksum: HTTP ${status} ${checksumUrl}`);
  }
  const expected = parseChecksum(body);
  const actual = (await sha256File(filePath)).toLowerCase();
  if (!crypto.timingSafeEqual(Buffer.from(expected, 'hex'), Buffer.from(actual, 'hex'))) {
    throw new Error(`checksum mismatch: expected ${expected}, got ${actual}`);
  }
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
  return `devcontainer-cli upgrade-cli — replace the current binary with the latest release

Usage:
  devcontainer-cli upgrade-cli [--check] [--force]

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

  console.error(
    chalk.gray(
      "Note: 'devcontainer-cli update' now manages container images. Self-update lives at 'devcontainer-cli upgrade-cli'.",
    ),
  );

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
  const { binary: binaryUrl, checksum: checksumUrl } = resolveAssetUrl(release, triplet, ext);
  console.log(chalk.gray(`Downloading ${binaryUrl}`));

  const tmpDir = fs.mkdtempSync(path.join(path.dirname(process.execPath), '.devcontainer-cli-'));
  const tmpPath = path.join(tmpDir, 'binary');
  try {
    await downloadToFile(binaryUrl, tmpPath);
    const stat = fs.statSync(tmpPath);
    if (stat.size === 0) throw new Error('downloaded file is empty');
    console.log(chalk.gray('Verifying checksum...'));
    await verifyChecksum(tmpPath, checksumUrl);
    replaceBinary(tmpPath);
  } catch (e) {
    try { fs.unlinkSync(tmpPath); } catch { /* noop */ }
    throw e;
  } finally {
    try { fs.rmdirSync(tmpDir); } catch { /* noop */ }
  }

  try {
    const refreshed = refreshInstalledCompletions(process.execPath);
    if (refreshed.length) console.log(chalk.gray(`Refreshed completion: ${refreshed.join(', ')}`));
  } catch (e) {
    console.error(chalk.yellow(`Completion refresh skipped: ${(e as Error).message}`));
  }

  console.log(chalk.green(`Updated to ${latest}. Restart any running session to use the new binary.`));
}
