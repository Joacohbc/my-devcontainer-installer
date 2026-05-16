import type { ComposeService } from '../../types.js';

export const redisService: ComposeService = {
  id: 'redis',
  label: 'Redis 7.4',
  volumes: ['redis_data'],
  render() {
    return {
      image: 'redis:7.4-alpine',
      container_name: 'redis',
      volumes: ['redis_data:/data'],
      networks: ['local-network'],
    };
  },
};
