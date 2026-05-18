import { dockerfileModules, getDockerfileModule } from './registry.js';
import type { DockerfileCategory, DockerfileModule, SelectedModule } from './types.js';

const CATEGORY_ORDER: DockerfileCategory[] = ['base', 'infra', 'lang', 'runtime', 'db', 'cleanup'];

export interface ResolvedModule {
  module: DockerfileModule;
  options: Record<string, unknown>;
}

export class ResolverError extends Error {}

export function resolveDockerfileModules(selected: SelectedModule[]): ResolvedModule[] {
  const byId = new Map<string, Record<string, unknown>>();

  for (const m of dockerfileModules) {
    if (m.always) byId.set(m.id, {});
  }

  for (const sel of selected) {
    const mod = getDockerfileModule(sel.id);
    if (!mod) throw new ResolverError(`Unknown module: ${sel.id}`);
    byId.set(sel.id, sel.options ?? {});
  }

  const stack = [...byId.keys()];
  while (stack.length > 0) {
    const id = stack.pop()!;
    const mod = getDockerfileModule(id)!;
    for (const req of mod.requires ?? []) {
      if (!byId.has(req)) {
        if (!getDockerfileModule(req)) {
          throw new ResolverError(`Module ${id} requires unknown module ${req}`);
        }
        byId.set(req, {});
        stack.push(req);
      }
    }
  }

  for (const id of byId.keys()) {
    const mod = getDockerfileModule(id)!;
    for (const conflict of mod.conflicts ?? []) {
      if (byId.has(conflict)) {
        throw new ResolverError(`Module ${id} conflicts with ${conflict}`);
      }
    }
  }

  const resolved: ResolvedModule[] = [];
  for (const cat of CATEGORY_ORDER) {
    for (const mod of dockerfileModules) {
      if (mod.category === cat && byId.has(mod.id)) {
        resolved.push({ module: mod, options: byId.get(mod.id)! });
      }
    }
  }
  return resolved;
}
