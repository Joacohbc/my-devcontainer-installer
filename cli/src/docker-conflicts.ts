import { spawnSync } from 'child_process';
import { LABEL_PROJECT } from '@/labels.js';
import { plannedComposeNames } from '@/generator.js';
import { projectId } from '@/labels.js';
import type { DevcontainerConfig } from '@/types.js';

export interface Conflict {
  kind: 'container' | 'network';
  name: string;
  owner: string;
}

function dockerAvailable(): boolean {
  const r = spawnSync('docker', ['version', '--format', '{{.Client.Version}}'], {
    encoding: 'utf8',
  });
  return r.status === 0;
}

function listExisting(kind: 'container' | 'network', project: string): Map<string, string> {
  const args =
    kind === 'container'
      ? ['container', 'ls', '-a', '--format', `{{.Names}}\t{{.Label "${LABEL_PROJECT}"}}`]
      : ['network', 'ls', '--format', `{{.Name}}\t{{.Label "${LABEL_PROJECT}"}}`];
  const r = spawnSync('docker', args, { encoding: 'utf8' });
  const out = new Map<string, string>();
  if (r.status !== 0) return out;
  for (const line of String(r.stdout ?? '').split('\n')) {
    if (!line.trim()) continue;
    const [name, label] = line.split('\t');
    out.set(name, label ?? '');
  }
  return out;
}

export function findConflicts(config: DevcontainerConfig): Conflict[] {
  if (!dockerAvailable()) return [];
  const plan = plannedComposeNames(config);
  const project = projectId(config);
  const conflicts: Conflict[] = [];

  const existingContainers = listExisting('container', project);
  for (const name of plan.containers) {
    const owner = existingContainers.get(name);
    if (owner === undefined) continue;
    if (owner === project) continue;
    conflicts.push({ kind: 'container', name, owner: owner || '(no label)' });
  }

  const existingNetworks = listExisting('network', project);
  const owner = existingNetworks.get(plan.network);
  if (owner !== undefined && owner !== project) {
    conflicts.push({ kind: 'network', name: plan.network, owner: owner || '(no label)' });
  }

  return conflicts;
}
