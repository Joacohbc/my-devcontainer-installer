import * as fs from 'fs';
import * as path from 'path';
import { CONFIG_FILE, type DevcontainerConfig } from './types.js';
import { sanitizeDockerName } from './validators.js';

export function configPath(cwd: string = process.cwd()): string {
  return path.join(cwd, CONFIG_FILE);
}

export function loadConfig(cwd: string = process.cwd()): DevcontainerConfig | null {
  const p = configPath(cwd);
  if (!fs.existsSync(p)) return null;
  try {
    const raw = fs.readFileSync(p, 'utf8');
    const parsed = JSON.parse(raw) as Partial<DevcontainerConfig> & DevcontainerConfig;
    if (!parsed.workspace) {
      parsed.workspace = sanitizeDockerName(path.basename(cwd));
    }
    // Compat: old configs used 'custom' or 'standalone' — both map to 'local-cached'.
    if (!parsed.mode || parsed.mode === ('custom' as string) || parsed.mode === ('standalone' as string)) {
      parsed.mode = 'local-cached';
    }
    return parsed;
  } catch (e) {
    throw new Error(`Failed to parse ${CONFIG_FILE}: ${(e as Error).message}`);
  }
}

export function saveConfig(config: DevcontainerConfig, cwd: string = process.cwd()): void {
  fs.writeFileSync(configPath(cwd), JSON.stringify(config, null, 2) + '\n');
}

export function defaultConfig(cwd: string = process.cwd()): DevcontainerConfig {
  const workspace = sanitizeDockerName(path.basename(cwd));
  return {
    mode: 'local-cached',
    image: `${workspace}:local`,
    workspace,
    dockerfile: { modules: [] },
    compose: { services: [], subnet: '172.25.0.0/28' },
    env: {},
  };
}
