import { dockerCapture } from '@/infra/docker.js';
import chalk from 'chalk';
import { select } from '@/infra/prompts.js';
import { LABEL_MANAGED } from '@/core/labels.js';
import { SSH_DEFAULTS } from '@/infra/ssh-defaults.js';

export interface ManagedContainer {
  name: string;
  image: string;
  status: string;
  state: string;
}

export interface DockerContainer extends ManagedContainer {
  /** true when the container carries the CLI managed label (i.e. a devcontainer). */
  managed: boolean;
}

export function containerWorkspace(containerName: string): string | null {
  const suffix = `-${SSH_DEFAULTS.serviceName}`;
  return containerName.endsWith(suffix) ? containerName.slice(0, -suffix.length) : null;
}

export function listManagedContainers(): ManagedContainer[] {
  // dockerCapture throws via ensureDocker() when docker is absent. Callers
  // (shell completion, interactive picker) expect a graceful empty list in
  // that case, so swallow the failure here.
  let r: { status: number; stdout: string; stderr: string };
  try {
    r = dockerCapture([
      'ps',
      '-a',
      '--filter',
      `label=${LABEL_MANAGED}=true`,
      '--format',
      '{{json .}}',
    ]);
  } catch {
    return [];
  }
  if (r.status !== 0 || !r.stdout.trim()) return [];
  return r.stdout
    .trim()
    .split('\n')
    .flatMap((line) => {
      try {
        const obj = JSON.parse(line) as Record<string, string>;
        const name = obj['Names'] ?? '';
        if (!name) return [];
        return [{ name, image: obj['Image'] ?? '', status: obj['Status'] ?? '', state: obj['State'] ?? '' }];
      } catch {
        return [];
      }
    });
}

export function listAllContainers(): DockerContainer[] {
  // Lists running containers regardless of the managed label, flagging which
  // ones are CLI-managed devcontainers. Used by port-forward so the user can
  // tunnel into any container, not just devcontainers. Swallows docker errors
  // the same way listManagedContainers does.
  let r: { status: number; stdout: string; stderr: string };
  try {
    r = dockerCapture(['ps', '--format', '{{json .}}']);
  } catch {
    return [];
  }
  if (r.status !== 0 || !r.stdout.trim()) return [];
  return r.stdout
    .trim()
    .split('\n')
    .flatMap((line) => {
      try {
        const obj = JSON.parse(line) as Record<string, string>;
        const name = obj['Names'] ?? '';
        if (!name) return [];
        const labels = obj['Labels'] ?? '';
        const managed = labels
          .split(',')
          .some((l) => l.trim() === `${LABEL_MANAGED}=true`);
        return [
          {
            name,
            image: obj['Image'] ?? '',
            status: obj['Status'] ?? '',
            state: obj['State'] ?? '',
            managed,
          },
        ];
      } catch {
        return [];
      }
    });
}

export function statusLabel(c: ManagedContainer): string {
  const text = c.status || c.state;
  if (c.state === 'running') return chalk.green(text);
  if (c.state === 'exited') return chalk.red(text);
  return chalk.yellow(text);
}

export async function pickManagedContainer(
  prompt: string,
  opts: { interactive: boolean; assumeYes?: boolean },
): Promise<ManagedContainer> {
  const containers = listManagedContainers();
  if (containers.length === 0) {
    throw new Error(
      "No devcontainer-cli managed containers found. Run 'devcontainer-cli' first to create one.",
    );
  }
  if (containers.length === 1 || opts.assumeYes) {
    const c = containers[0];
    console.log(chalk.cyan(`Using container: ${c.name}  ${chalk.gray(c.image)}  ${statusLabel(c)}`));
    return c;
  }
  if (!opts.interactive) {
    throw new Error(
      'Multiple CLI-managed containers found. Specify a container explicitly or run interactively.',
    );
  }
  const maxName = Math.max(...containers.map((c) => c.name.length));
  const maxImage = Math.max(...containers.map((c) => c.image.length));
  const chosen = await select(
    'container',
    prompt,
    containers.map((c) => ({
      name: c.name,
      message: `${c.name.padEnd(maxName)}  ${chalk.gray(c.image.padEnd(maxImage))}  ${statusLabel(c)}`,
    })),
  );
  return containers.find((c) => c.name === chosen)!;
}
