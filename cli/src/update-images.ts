import * as fs from 'fs';
import * as path from 'path';
import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { loadConfig } from '@/config.js';
import { listEntries, recordEntry, removeEntry } from '@/image-registry.js';
import { resolveRemoteImage } from '@/generator.js';
import type { DevcontainerConfig } from '@/types.js';

export interface UpdateImagesFlags {
  all: boolean;
  pull: boolean;
  rebuild: boolean;
  help: boolean;
}

export function parseUpdateFlags(argv: string[]): UpdateImagesFlags {
  const flags: UpdateImagesFlags = {
    all: false,
    pull: false,
    rebuild: false,
    help: false,
  };
  for (const a of argv) {
    switch (a) {
      case '--all':
        flags.all = true;
        break;
      case '--pull':
        flags.pull = true;
        break;
      case '--rebuild':
        flags.rebuild = true;
        break;
      case '-h':
      case '--help':
        flags.help = true;
        break;
      default:
        throw new Error(`Unknown flag for update: ${a}`);
    }
  }
  return flags;
}

export function updateHelp(): string {
  return `devcontainer-cli update — update container images

Usage:
  devcontainer-cli update              Update images for the project in the current dir
  devcontainer-cli update --all        Update images for every recorded project
  devcontainer-cli update --pull       Force pull for remote images
  devcontainer-cli update --rebuild    Force rebuild for custom/local-cached images

Flags:
  --all       Update images for every project tracked in images.json
  --pull      Always pull (no-op for custom)
  --rebuild   Always rebuild (no-op for remote)
  -h, --help  Show this help

Note: To update the CLI binary itself, run 'devcontainer-cli upgrade-cli'.
`;
}

const log = (s: string) => console.log(`${chalk.blue.bold('==>')} ${s}`);
const ok = (s: string) => console.log(`${chalk.green.bold('✓')}   ${s}`);
const warn = (s: string) => console.log(`${chalk.yellow.bold('!')}   ${s}`);

function runInherit(cmd: string, args: string[], cwd?: string): number {
  const r = spawnSync(cmd, args, { stdio: 'inherit', cwd });
  return r.status ?? -1;
}

function buildDirFor(projectDir: string, workspace: string): string {
  return path.join(projectDir, `.dc_${workspace}`, 'build');
}

function composeFileFor(projectDir: string, workspace: string): string {
  return path.join(buildDirFor(projectDir, workspace), 'docker-compose.yml');
}

interface UpdateResult {
  ok: boolean;
  image: string;
}

async function updateOne(
  projectDir: string,
  config: DevcontainerConfig,
  flags: UpdateImagesFlags,
): Promise<UpdateResult> {
  const workspace = config.workspace;
  log(`Updating '${workspace}' (mode=${config.mode}) at ${projectDir}`);

  if (config.mode === 'remote') {
    if (!config.remote) {
      warn('Remote config missing — skipping.');
      return { ok: false, image: config.image };
    }
    const image = resolveRemoteImage(
      config.remote.variant,
      undefined,
      config.remote.registry,
    );
    const status = runInherit('docker', ['pull', image]);
    if (status !== 0) return { ok: false, image };
    ok(`Pulled ${image}`);
    return { ok: true, image };
  }

  // custom or local-cached → docker compose build --pull
  const composeFile = composeFileFor(projectDir, workspace);
  if (!fs.existsSync(composeFile)) {
    warn(`No compose file at ${composeFile} — skipping.`);
    return { ok: false, image: config.image };
  }
  const args = ['compose', '-f', composeFile, 'build'];
  if (!flags.rebuild || flags.pull) args.push('--pull');
  const status = runInherit('docker', args, projectDir);
  if (status !== 0) return { ok: false, image: config.image };
  ok(`Rebuilt ${config.image}`);
  return { ok: true, image: config.image };
}

async function updateAll(flags: UpdateImagesFlags): Promise<void> {
  const entries = listEntries();
  if (entries.length === 0) {
    warn('No projects recorded yet. Generate at least one project first.');
    return;
  }
  let okCount = 0;
  let skipCount = 0;
  let failCount = 0;
  for (const e of entries) {
    if (!fs.existsSync(e.projectDir)) {
      warn(`Project directory missing: ${e.projectDir} — removing from registry.`);
      removeEntry(e.projectDir);
      skipCount++;
      continue;
    }
    const cfg = loadConfig(e.projectDir);
    if (!cfg) {
      warn(`No devcontainer.config.json at ${e.projectDir} — skipping.`);
      skipCount++;
      continue;
    }
    try {
      const r = await updateOne(e.projectDir, cfg, flags);
      if (r.ok) {
        recordEntry({
          ...e,
          mode: cfg.mode,
          image: r.image,
          lastUpdated: new Date().toISOString(),
        });
        okCount++;
      } else {
        failCount++;
      }
    } catch (err) {
      console.error(chalk.red(`  ${(err as Error).message}`));
      failCount++;
    }
  }
  console.log(
    chalk.gray(`\n--- ${okCount} updated, ${skipCount} skipped, ${failCount} failed`),
  );
}

export async function runUpdateImages(argv: string[]): Promise<void> {
  const flags = parseUpdateFlags(argv);
  if (flags.help) {
    console.log(updateHelp());
    return;
  }
  if (flags.all) {
    await updateAll(flags);
    return;
  }
  const cwd = process.cwd();
  const config = loadConfig(cwd);
  if (!config) {
    throw new Error(
      `No devcontainer.config.json found in ${cwd}. Run 'devcontainer-cli' to generate one first, or pass --all to update every recorded project.`,
    );
  }
  const r = await updateOne(cwd, config, flags);
  if (r.ok) {
    recordEntry({
      projectDir: cwd,
      workspace: config.workspace,
      mode: config.mode,
      image: r.image,
      variant: config.remote?.variant,
      fingerprint: config.fingerprint,
      createdAt: new Date().toISOString(),
      lastUpdated: new Date().toISOString(),
    });
  } else {
    throw new Error('Update failed.');
  }
}
