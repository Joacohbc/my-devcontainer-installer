import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { exec } from 'child_process';
import enquirer from 'enquirer';
import chalk from 'chalk';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const templatesDir = path.join(__dirname, '../templates');

const tools = [
  { name: 'dod.Dockerfile', message: 'Docker-outside-Docker (DoD)' },
  { name: 'java.Dockerfile', message: 'Java (JDK 11 & 17 + Maven)' },
  { name: 'python.Dockerfile', message: 'Python (Python 3 + pip)' },
  { name: 'sqlite.Dockerfile', message: 'SQLite' },
  { name: 'go.Dockerfile', message: 'Go' },
  { name: 'dbclients.Dockerfile', message: 'Database Clients (Postgres, Redis, MongoDB)' },
  { name: 'nodejs.Dockerfile', message: 'Node.js (NVM, Node LTS, PNPM)' },
  { name: 'bun.Dockerfile', message: 'Bun' },
];

async function main() {
  console.log(chalk.green.bold('\n🚀 DevContainer Dockerfile Builder\n'));

  const { selectedTools } = await enquirer.prompt<{ selectedTools: string[] }>({
    type: 'multiselect',
    name: 'selectedTools',
    message: 'Select the tools you want to install (Space to select, Enter to confirm):',
    choices: tools,
  });

  generateDockerfile(selectedTools);
  console.log(chalk.green('\n✅ Dockerfile generated successfully!'));

  const { buildNow } = await enquirer.prompt<{ buildNow: boolean }>({
    type: 'confirm',
    name: 'buildNow',
    message: "Do you want to run 'docker compose build' now?",
    initial: true,
  });

  if (buildNow) {
    console.log(chalk.yellow('\n⏳ Building the image. Please wait...\n'));
    await executeBuild();
  } else {
    console.log(chalk.green.bold('\n✨ All done! Have a great day.\n'));
  }
}

function generateDockerfile(selectedFiles: string[]) {
  try {
    const baseContent = fs.readFileSync(path.join(templatesDir, 'base.Dockerfile'), 'utf-8');
    const cleanupContent = fs.readFileSync(path.join(templatesDir, 'cleanup.Dockerfile'), 'utf-8');

    let finalContent = baseContent + '\n';

    selectedFiles.forEach(file => {
      const content = fs.readFileSync(path.join(templatesDir, file), 'utf-8');
      finalContent += content + '\n';
    });

    finalContent += cleanupContent + '\n';

    // Write to the main Dockerfile in the root of the repo
    const rootDockerfilePath = path.join(__dirname, '../../Dockerfile');
    fs.writeFileSync(rootDockerfilePath, finalContent);
  } catch (e) {
    console.error(chalk.red('Error generating Dockerfile:'), e);
  }
}

function executeBuild(): Promise<void> {
  return new Promise((resolve, reject) => {
    // We execute from the repo root
    const rootDir = path.join(__dirname, '../../');
    const child = exec('docker compose build', { cwd: rootDir });

    child.stdout?.pipe(process.stdout);
    child.stderr?.pipe(process.stderr);

    child.on('close', (code) => {
      if (code !== 0) {
        console.error(chalk.red(`\n❌ Build failed with exit code ${code}\n`));
      } else {
        console.log(chalk.green.bold('\n✅ Build completed successfully!\n'));
      }
      resolve();
    });
  });
}

main().catch(console.error);
