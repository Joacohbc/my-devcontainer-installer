import type { DevcontainerConfig } from './types.js';

export const LABEL_NAMESPACE = 'dev.devcontainer-installer';
export const LABEL_MANAGED = `${LABEL_NAMESPACE}.managed`;
export const LABEL_PROJECT = `${LABEL_NAMESPACE}.project`;
export const LABEL_VERSION = `${LABEL_NAMESPACE}.version`;

export const SCHEMA_VERSION = '1';

export function projectId(config: DevcontainerConfig): string {
  return config.image.replace(/[:@/]/g, '_');
}

export function dockerfileLabelBlock(config: DevcontainerConfig): string {
  const project = projectId(config);
  return [
    '# Standard labels — used by the CLI to identify, purge and update images.',
    `LABEL ${LABEL_MANAGED}="true" \\`,
    `      ${LABEL_PROJECT}="${project}" \\`,
    `      ${LABEL_VERSION}="${SCHEMA_VERSION}"`,
  ].join('\n');
}

export function composeLabels(config: DevcontainerConfig): Record<string, string> {
  return {
    [LABEL_MANAGED]: 'true',
    [LABEL_PROJECT]: projectId(config),
    [LABEL_VERSION]: SCHEMA_VERSION,
  };
}
