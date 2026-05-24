import { spawnSync as cpSpawnSync } from 'child_process';

export const spawner = {
  spawnSync: cpSpawnSync,
};

let isDockerAvailableCache: boolean | null = null;

export function isDockerAvailable(): boolean {
  if (isDockerAvailableCache !== null) {
    return isDockerAvailableCache;
  }
  try {
    const r = spawner.spawnSync('docker', ['version', '--format', '{{.Client.Version}}'], {
      encoding: 'utf8',
    });
    isDockerAvailableCache = r.status === 0;
  } catch {
    isDockerAvailableCache = false;
  }
  return isDockerAvailableCache;
}

export function resetDockerCache(): void {
  isDockerAvailableCache = null;
}

export function ensureDocker(): void {
  if (!isDockerAvailable()) {
    throw new Error('Docker is not installed or the daemon is not running.');
  }
}

export function dockerInherit(args: string[]): number {
  ensureDocker();
  const r = spawner.spawnSync('docker', args, { stdio: 'inherit' });
  return r.status ?? 1;
}

export function dockerCapture(args: string[]): { status: number; stdout: string; stderr: string } {
  ensureDocker();
  const r = spawner.spawnSync('docker', args, { encoding: 'utf8' });
  return {
    status: r.status ?? 1,
    stdout: r.stdout ?? '',
    stderr: r.stderr ?? '',
  };
}

export interface DockerComposeOptions {
  cwd?: string;
  stdio?: 'inherit' | 'pipe';
  env?: Record<string, string>;
}

export function dockerCompose(composeFile: string, args: string[], opts?: DockerComposeOptions): number {
  ensureDocker();
  const cmdArgs = ['compose', '-f', composeFile, ...args];
  const r = spawner.spawnSync('docker', cmdArgs, {
    stdio: opts?.stdio ?? 'inherit',
    cwd: opts?.cwd,
    env: opts?.env ? { ...process.env, ...opts.env } : process.env,
  });
  return r.status ?? 1;
}

export function dockerComposeOrThrow(composeFile: string, args: string[], opts?: DockerComposeOptions): void {
  const status = dockerCompose(composeFile, args, opts);
  if (status !== 0) {
    throw new Error(`docker compose -f ${composeFile} ${args.join(' ')} failed.`);
  }
}
