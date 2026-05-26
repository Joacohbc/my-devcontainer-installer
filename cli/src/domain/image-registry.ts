import * as crypto from 'crypto';
import * as fs from 'fs';
import * as path from 'path';
import { dockerCapture } from '@/infra/docker.js';
import { globalConfigDir } from '@/domain/global-config.js';
import type { BuildMode, RemoteVariant, DevcontainerConfig } from '@/core/types.js';

export interface ImageEntry {
  projectDir: string;
  workspace: string;
  mode: BuildMode;
  image: string;
  fingerprint?: string;
  variant?: RemoteVariant;
  createdAt: string;
  lastUpdated: string;
}

interface RegistryFile {
  entries: ImageEntry[];
}

export function imageRegistryPath(): string {
  return path.join(globalConfigDir(), 'images.json');
}

export function loadRegistry(): ImageEntry[] {
  const p = imageRegistryPath();
  if (!fs.existsSync(p)) return [];
  try {
    const raw = fs.readFileSync(p, 'utf8');
    const parsed = JSON.parse(raw) as RegistryFile;
    return Array.isArray(parsed?.entries) ? parsed.entries : [];
  } catch {
    return [];
  }
}

export function saveRegistry(entries: ImageEntry[]): void {
  const dir = globalConfigDir();
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(
    imageRegistryPath(),
    JSON.stringify({ entries }, null, 2) + '\n',
  );
}

export function recordEntry(e: ImageEntry): void {
  const entries = loadRegistry();
  const idx = entries.findIndex((x) => x.projectDir === e.projectDir);
  if (idx >= 0) {
    entries[idx] = { ...entries[idx], ...e, lastUpdated: e.lastUpdated };
  } else {
    entries.push(e);
  }
  saveRegistry(entries);
}

export function removeEntry(projectDir: string): boolean {
  const entries = loadRegistry();
  const idx = entries.findIndex((x) => x.projectDir === projectDir);
  if (idx < 0) return false;
  entries.splice(idx, 1);
  saveRegistry(entries);
  return true;
}

export function findByFingerprint(fingerprint: string): ImageEntry | undefined {
  return loadRegistry().find((e) => e.fingerprint === fingerprint);
}

export function listEntries(): ImageEntry[] {
  return loadRegistry();
}

// Strip LABEL blocks so label-only changes (e.g. timestamp metadata) don't
// invalidate the cache. A LABEL block is a single LABEL line or a continued
// multi-line statement (using `\` line continuations).
function stripLabelBlocks(dockerfile: string): string {
  const lines = dockerfile.split('\n');
  const out: string[] = [];
  let inLabel = false;
  for (const line of lines) {
    const trimmed = line.trim();
    if (!inLabel) {
      if (/^LABEL\b/i.test(trimmed)) {
        inLabel = trimmed.endsWith('\\');
        continue;
      }
      out.push(line);
    } else {
      inLabel = trimmed.endsWith('\\');
    }
  }
  return out.join('\n');
}

function sha256Hex(s: string | Buffer): string {
  return crypto.createHash('sha256').update(s).digest('hex');
}

export function computeFingerprint(
  dockerfileContent: string,
  copyFileContents: Record<string, string>,
  selectedModuleIds: string[] = [],
): string {
  const normalizedDockerfile = stripLabelBlocks(dockerfileContent)
    .replace(/\r\n/g, '\n')
    .trim();
  const parts: string[] = [normalizedDockerfile];

  const sortedNames = Object.keys(copyFileContents).sort();
  for (const name of sortedNames) {
    parts.push(`${path.basename(name)}\0${sha256Hex(copyFileContents[name])}`);
  }
  parts.push(`MODULES\0${[...selectedModuleIds].sort().join(',')}`);

  return sha256Hex(parts.join('\n'));
}

export function fingerprintTag(fingerprint: string): string {
  return `devcontainer-cli/${fingerprint.slice(0, 12)}:latest`;
}

export function localImageExists(image: string): boolean {
  const r = dockerCapture(['image', 'inspect', image]);
  return r.status === 0;
}

export function recordProject(projectDir: string, config: DevcontainerConfig, customImage?: string): void {
  try {
    const now = new Date().toISOString();
    recordEntry({
      projectDir,
      workspace: config.workspace,
      mode: config.mode,
      image: customImage ?? config.image,
      variant: config.remote?.variant,
      fingerprint: config.fingerprint,
      createdAt: now,
      lastUpdated: now,
    });
  } catch {
    // Non-fatal
  }
}
