import * as fs from 'fs';
import { spawnSync } from 'child_process';
import chalk from 'chalk';
import { listEntries } from './image-registry.js';
import { confirm } from './prompts.js';

export interface PruneFlags {
  all: boolean;
  yes: boolean;
  interactive: boolean;
  help: boolean;
}

export function parsePruneFlags(argv: string[]): PruneFlags {
  const flags: PruneFlags = { all: false, yes: false, interactive: true, help: false };
  for (const a of argv) {
    switch (a) {
      case '-h':
      case '--help':
        flags.help = true;
        break;
      case '--all':
        flags.all = true;
        break;
      case '-y':
      case '--yes':
        flags.yes = true;
        break;
      case '--no-interactive':
        flags.interactive = false;
        break;
      default:
        throw new Error(`Unknown flag for prune: ${a}`);
    }
  }
  return flags;
}

export function pruneHelp(): string {
  return `devcontainer-cli prune — remove devcontainer-cli images whose projects no longer exist

Usage:
  devcontainer-cli prune [flags]

By default removes only orphan images (project directory is gone).
Use --all to remove every devcontainer-cli/* image regardless.

Flags:
  --all              Remove ALL devcontainer-cli/* images, not just orphans
  -y, --yes          Skip confirmation prompt
  --no-interactive   Non-interactive mode
  -h, --help         Show this help
`;
}

interface LocalImage {
  ref: string;
  id: string;
}

function listCliImages(): LocalImage[] {
  const r = spawnSync('docker', [
    'images',
    '--filter', 'reference=devcontainer-cli/*',
    '--format', '{{.Repository}}:{{.Tag}}\t{{.ID}}',
  ], { encoding: 'utf8' });
  if ((r.status ?? -1) !== 0 || !r.stdout.trim()) return [];
  return r.stdout.trim().split('\n').map((line) => {
    const [ref, id] = line.split('\t');
    return { ref: ref.trim(), id: id.trim() };
  });
}

function removeImage(ref: string): boolean {
  const r = spawnSync('docker', ['rmi', ref], { stdio: 'inherit' });
  return (r.status ?? -1) === 0;
}

export async function runPrune(argv: string[]): Promise<void> {
  const flags = parsePruneFlags(argv);
  if (flags.help) { console.log(pruneHelp()); return; }

  const localImages = listCliImages();
  if (localImages.length === 0) {
    console.log(chalk.gray('No devcontainer-cli/* images found locally.'));
    return;
  }

  let toRemove: LocalImage[];

  if (flags.all) {
    toRemove = localImages;
  } else {
    // Orphan: image is tracked but its project dir no longer exists,
    // OR the image isn't tracked at all in the registry.
    const entries = listEntries();
    const trackedImages = new Set(entries.map((e) => e.image));
    const liveProjects = new Set(
      entries.filter((e) => fs.existsSync(e.projectDir)).map((e) => e.image),
    );
    toRemove = localImages.filter(
      (img) => !trackedImages.has(img.ref) || !liveProjects.has(img.ref),
    );
  }

  if (toRemove.length === 0) {
    console.log(chalk.gray('No orphan devcontainer-cli/* images found.'));
    console.log(chalk.gray('Use --all to remove every devcontainer-cli/* image.'));
    return;
  }

  console.log(chalk.yellow(`\nImages to remove (${toRemove.length}):`));
  for (const img of toRemove) {
    console.log(chalk.gray(`  ${img.ref}  (${img.id})`));
  }
  console.log('');

  if (!flags.yes && flags.interactive) {
    const proceed = await confirm('prune', 'Remove these images?', false);
    if (!proceed) { console.log(chalk.yellow('Cancelled.')); return; }
  }

  let ok = 0;
  let fail = 0;
  for (const img of toRemove) {
    if (removeImage(img.ref)) {
      ok++;
    } else {
      console.log(chalk.red(`  ✗ Failed to remove ${img.ref} (container may be running)`));
      fail++;
    }
  }

  console.log(chalk.green.bold(`\n✅ Removed ${ok} image(s).`) + (fail ? chalk.red(` ${fail} failed.`) : ''));
}
