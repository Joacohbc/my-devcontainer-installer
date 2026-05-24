import { dockerCapture } from '@/docker.js';
import chalk from 'chalk';
import { select } from '@/prompts.js';
import { LABEL_MANAGED } from '@/labels.js';
import { SSH_DEFAULTS } from '@/ssh-defaults.js';

export interface ManagedContainer {
  name: string;
  image: string;
  status: string;
  state: string;
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

function statusLabel(c: ManagedContainer): string {
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
