import type { ComposeService } from '../../types.js';

export const postgresService: ComposeService = {
  id: 'postgres',
  label: 'PostgreSQL',
  volumes: ['postgres_data'],
  options: [
    {
      id: 'version',
      label: 'PostgreSQL version',
      type: 'select',
      choices: [
        { value: '17-alpine', label: 'PostgreSQL 17' },
        { value: '18-alpine', label: 'PostgreSQL 18 (latest)' },
        { value: '16-alpine', label: 'PostgreSQL 16' },
      ],
      default: '17-alpine',
    },
  ],
  render({ options }) {
    const version = (options.version as string) || '17-alpine';
    return {
      image: `postgres:${version}`,
      container_name: 'postgres',
      environment: {
        POSTGRES_USER: 'devuser',
        POSTGRES_PASSWORD: 'devpass',
        POSTGRES_DB: 'devdb',
      },
      volumes: ['postgres_data:/var/lib/postgresql/data'],
      networks: ['local-network'],
    };
  },
};
