package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var TunnelService = &ServiceSpec{
	ID:         "tunnel",
	Label:      "Cloudflare Tunnel (cloudflared)",
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: "TUNNEL_TOKEN", Prompt: "Cloudflare Tunnel Token (TUNNEL_TOKEN)"},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		return &ServiceDef{
			Image:         "cloudflare/cloudflared:latest",
			ContainerName: "cloudflared_tunnel",
			Restart:       "unless-stopped",
			Command:       "tunnel run",
			Environment:   []string{"TUNNEL_TOKEN=${TUNNEL_TOKEN}"},
			Networks:      []string{"local-network"},
		}
	},
}
