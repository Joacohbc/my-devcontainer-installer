import type { ComposeService } from '../../types.js';

export const postgresService: ComposeService = {
  id: 'postgres',
  label: 'PostgreSQL 17',
  volumes: ['postgres_data'],
  render() {
    return {
      image: 'postgres:17-alpine',
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
