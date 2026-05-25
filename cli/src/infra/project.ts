import * as fs from 'fs';
import * as path from 'path';
import { loadConfig } from '@/domain/config.js';
import { sanitizeDockerName } from '@/domain/validators.js';
import type { DevcontainerConfig } from '@/core/types.js';

export function resolveWorkspace(cwd: string, config?: DevcontainerConfig | null): string {
  const cfg = config !== undefined ? config : loadConfig(cwd);
  return cfg?.workspace ?? sanitizeDockerName(path.basename(cwd));
}

export interface ProjectPaths {
  projectDir: string;
  buildDir: string;
  composeFile: string;
  dockerfilePath: string;
  envPath: string;
}

export function projectPaths(cwd: string, workspace: string): ProjectPaths {
  const projectDir = path.join(cwd, `.dc_${workspace}`);
  const buildDir = path.join(projectDir, 'build');
  return {
    projectDir,
    buildDir,
    composeFile: path.join(buildDir, 'docker-compose.yml'),
    dockerfilePath: path.join(buildDir, 'Dockerfile'),
    envPath: path.join(buildDir, '.env'),
  };
}

export function resolveProjectComposeFile(cwd: string): string {
  const workspace = resolveWorkspace(cwd);
  const paths = projectPaths(cwd, workspace);
  if (!fs.existsSync(paths.composeFile)) {
    throw new Error(`No compose file found at ${paths.composeFile}. Run 'devcontainer-cli' to generate one first.`);
  }
  return paths.composeFile;
}
