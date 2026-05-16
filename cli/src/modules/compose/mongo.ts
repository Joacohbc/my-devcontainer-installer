import type { ComposeService } from '../../types.js';

export const mongoService: ComposeService = {
  id: 'mongo',
  label: 'MongoDB',
  volumes: ['mongo_data'],
  options: [
    {
      id: 'version',
      label: 'MongoDB version',
      type: 'select',
      choices: [
        { value: '8.0', label: 'MongoDB 8.0 (LTS)' },
        { value: '8.3', label: 'MongoDB 8.3 (latest)' },
        { value: '7.0', label: 'MongoDB 7.0' },
      ],
      default: '8.0',
    },
  ],
  render({ options }) {
    const version = (options.version as string) || '8.0';
    return {
      image: `mongo:${version}`,
      container_name: 'mongo',
      environment: {
        MONGO_INITDB_ROOT_USERNAME: 'devuser',
        MONGO_INITDB_ROOT_PASSWORD: 'devpass',
      },
      volumes: ['mongo_data:/data/db'],
      networks: ['local-network'],
    };
  },
};
