import type { ComposeService } from '@/core/types.js';

export const tunnelService: ComposeService = {
  id: 'tunnel',
  label: 'Cloudflare Tunnel (cloudflared)',
  requiresEnv: [{ name: 'TUNNEL_TOKEN', prompt: 'Cloudflare Tunnel Token (TUNNEL_TOKEN)' }],
  render() {
    return {
      image: 'cloudflare/cloudflared:latest',
      container_name: 'cloudflared_tunnel',
      restart: 'unless-stopped',
      command: 'tunnel run',
      environment: ['TUNNEL_TOKEN=${TUNNEL_TOKEN}'],
      networks: ['local-network'],
    };
  },
};
