import type { ComposeService, DockerfileModule } from '@/types.js';

import { baseModule } from '@/modules/dockerfile/base.js';
import { githubCliModule } from '@/modules/dockerfile/github-cli.js';
import { dodModule } from '@/modules/dockerfile/dod.js';
import { javaTemurinModule } from '@/modules/dockerfile/java-temurin.js';
import { javaOpenjdkModule } from '@/modules/dockerfile/java-openjdk.js';
import { pythonModule } from '@/modules/dockerfile/python.js';
import { sqliteModule } from '@/modules/dockerfile/sqlite.js';
import { goModule } from '@/modules/dockerfile/go.js';
import { dbclientsModule } from '@/modules/dockerfile/dbclients.js';
import { nodejsModule } from '@/modules/dockerfile/nodejs.js';
import { pnpmModule } from '@/modules/dockerfile/pnpm.js';
import { bunModule } from '@/modules/dockerfile/bun.js';
import { cleanupModule } from '@/modules/dockerfile/cleanup.js';
import { aiClisModule } from '@/modules/dockerfile/ai-clis.js';
import { tmuxModule } from '@/modules/dockerfile/tmux.js';

import { devcontainerService } from '@/modules/compose/devcontainer.js';
import { dockerSocketProxyService } from '@/modules/compose/docker-socket-proxy.js';
import { dindEngineService } from '@/modules/compose/dind-engine.js';
import { mongoService } from '@/modules/compose/mongo.js';
import { redisService } from '@/modules/compose/redis.js';
import { postgresService } from '@/modules/compose/postgres.js';
import { tunnelService } from '@/modules/compose/tunnel.js';

export const dockerfileModules: DockerfileModule[] = [
  baseModule,
  githubCliModule,
  dodModule,
  javaTemurinModule,
  javaOpenjdkModule,
  pythonModule,
  sqliteModule,
  goModule,
  dbclientsModule,
  nodejsModule,
  pnpmModule,
  bunModule,
  aiClisModule,
  tmuxModule,
  cleanupModule,
];

export const composeServices: ComposeService[] = [
  devcontainerService,
  dockerSocketProxyService,
  dindEngineService,
  mongoService,
  redisService,
  postgresService,
  tunnelService,
];

export function getDockerfileModule(id: string): DockerfileModule | undefined {
  return dockerfileModules.find((m) => m.id === id);
}

export function getComposeService(id: string): ComposeService | undefined {
  return composeServices.find((s) => s.id === id);
}
