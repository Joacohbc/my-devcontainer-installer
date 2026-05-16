import type { ComposeService } from '../../types.js';

export const mongoService: ComposeService = {
  id: 'mongo',
  label: 'MongoDB 8.0',
  volumes: ['mongo_data'],
  render() {
    return {
      image: 'mongo:8.0',
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
