import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

export interface GlobalConfig {
  registry?: string;
}

const DEFAULT_REGISTRY = 'ghcr.io/joacohbc/';

export function globalConfigDir(): string {
  if (process.platform === 'win32') {
    const appData = process.env.APPDATA;
    if (appData) return path.join(appData, 'devcontainer-cli');
    return path.join(os.homedir(), 'AppData', 'Roaming', 'devcontainer-cli');
  }
  const xdg = process.env.XDG_CONFIG_HOME;
  if (xdg) return path.join(xdg, 'devcontainer-cli');
  return path.join(os.homedir(), '.config', 'devcontainer-cli');
}

export function globalConfigPath(): string {
  return path.join(globalConfigDir(), 'config.json');
}

export function loadGlobalConfig(): GlobalConfig {
  const p = globalConfigPath();
  if (!fs.existsSync(p)) return {};
  try {
    const raw = fs.readFileSync(p, 'utf8');
    const parsed = JSON.parse(raw) as GlobalConfig;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

export function saveGlobalConfig(cfg: GlobalConfig): void {
  const dir = globalConfigDir();
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(globalConfigPath(), JSON.stringify(cfg, null, 2) + '\n');
}

function normalizeRegistry(url: string): string {
  const trimmed = url.trim();
  if (!trimmed) return trimmed;
  return trimmed.endsWith('/') ? trimmed : `${trimmed}/`;
}

export function resolveRegistry(flagOverride?: string, perProject?: string): string {
  if (flagOverride) return normalizeRegistry(flagOverride);
  if (perProject) return normalizeRegistry(perProject);
  const global = loadGlobalConfig().registry;
  if (global) return normalizeRegistry(global);
  return DEFAULT_REGISTRY;
}

export const REGISTRY_DEFAULT = DEFAULT_REGISTRY;
