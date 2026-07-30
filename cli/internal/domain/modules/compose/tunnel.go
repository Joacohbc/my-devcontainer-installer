package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var TunnelService = &ServiceSpec{
	ID:         types.ServiceTunnel,
	Label:      "Cloudflare Tunnel (cloudflared)",
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: "TUNNEL_TOKEN", Prompt: "Cloudflare Tunnel Token (TUNNEL_TOKEN)"},
	},
	Context: func(ctx RenderContext) *types.ContextSection {
		return &types.ContextSection{
			Title: "Cloudflare Tunnel (sibling container)",
			Body: ctxBody(
				"A `cloudflared` container runs alongside this one and can expose services on",
				"the project network to a public Cloudflare hostname. Routing is configured in",
				"the Cloudflare dashboard, not here.",
				"",
				"**It faces the public internet.** Treat anything reachable through it as",
				"published, and do not change its routing unless asked.",
			),
		}
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
