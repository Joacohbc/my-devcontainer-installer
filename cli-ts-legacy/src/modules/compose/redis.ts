import type { ComposeService } from '@/core/types.js';

export const redisService: ComposeService = {
  id: 'redis',
  label: 'Redis',
  volumes: ['redis_data'],
  options: [
    {
      id: 'version',
      label: 'Redis version',
      type: 'select',
      choices: [
        { value: '7.4-alpine', label: 'Redis 7.4 (BSD-licensed last)' },
        { value: '8.6-alpine', label: 'Redis 8.6 (latest, SSPL/RSALv2)' },
        { value: '8.0-alpine', label: 'Redis 8.0' },
      ],
      default: '7.4-alpine',
    },
  ],
  render({ options }) {
    const version = (options.version as string) || '7.4-alpine';
    return {
      image: `redis:${version}`,
      container_name: 'redis',
      volumes: ['redis_data:/data'],
      networks: ['local-network'],
    };
  },
};
