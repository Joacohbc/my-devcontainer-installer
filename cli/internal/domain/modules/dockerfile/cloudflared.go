package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// TunnelTokenEnv is the env var a Cloudflare connector token is passed through.
// Declared as RequiresEnv below so the generate wizard offers it; leaving it
// empty is a supported answer — the user then logs in from inside the container
// with `cloudflared tunnel login` instead of pre-seeding a token.
const TunnelTokenEnv = "TUNNEL_TOKEN"

var CloudflaredModule = &ModuleSpec{
	ID:         types.ModuleCloudflared,
	Label:      "Cloudflare Tunnel (cloudflared)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: TunnelTokenEnv, Prompt: "Cloudflare Tunnel token (leave empty to run 'cloudflared tunnel login' inside the container)"},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Cloudflare Tunnel (cloudflared)",
			Body: ctxBody(
				"`cloudflared` is installed in this container, so a tunnel terminates here and",
				"reaches `localhost` directly — no sibling container and no network hop.",
				"",
				"If `$"+TunnelTokenEnv+"` is set, the connector is already authorized and",
				"`cloudflared tunnel run` picks it up (routing is configured in the Cloudflare",
				"dashboard, not here). If it is empty, the tunnel needs a login first:",
				"`cloudflared tunnel login`, whose certificate lands in `~/.cloudflared`.",
				"",
				"**It publishes to the public internet.** Do not start a tunnel unless you were",
				"explicitly asked to.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## CLOUDFLARE TUNNEL (cloudflared)
##
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -nv -O /tmp/cloudflare-main.gpg https://pkg.cloudflare.com/cloudflare-main.gpg && \
    cat /tmp/cloudflare-main.gpg | tee /etc/apt/keyrings/cloudflare-main.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/cloudflare-main.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main" | tee /etc/apt/sources.list.d/cloudflared.list > /dev/null && \
    apt-get update && \
    apt-get install -y cloudflared && \
    %s
`, aptCleanup())
	},
}
