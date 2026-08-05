package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// TunnelTokenEnv is the env var a Cloudflare connector token is passed through.
// It keeps the name the removed "tunnel" compose service used, so a migrated
// project's existing .env keeps working untouched.
const TunnelTokenEnv = "TUNNEL_TOKEN"

var CloudflaredModule = &ModuleSpec{
	ID:         types.ModuleCloudflared,
	Label:      "Cloudflare Tunnel (cloudflared)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: TunnelTokenEnv, Prompt: "Cloudflare Tunnel token (optional; empty = log in or use a free quick tunnel in the container)"},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Cloudflare Tunnel (cloudflared)",
			Body: ctxBody(
				"`cloudflared` is installed in this container, so a tunnel terminates here and",
				"reaches `localhost` directly — no sibling container and no network hop.",
				"",
				"There are three ways to run one, in order of how much setup they need:",
				"",
				"- **Quick tunnel — free, no account, no login.**",
				"  `cloudflared tunnel --url http://localhost:<port>` prints a random",
				"  `https://<something>.trycloudflare.com` URL and starts serving that port",
				"  immediately. Nothing to configure and no `$"+TunnelTokenEnv+"` needed. The",
				"  URL is ephemeral: it dies with the process and is different every run, so it",
				"  is the right choice for a one-off demo or webhook test, not for a stable",
				"  address.",
				"- **Named tunnel with a token.** If `$"+TunnelTokenEnv+"` is set the connector",
				"  is already authorized and `cloudflared tunnel run` picks it up. Routing is",
				"  configured in the Cloudflare dashboard, not here.",
				"- **Named tunnel without a token.** Run `cloudflared tunnel login` first (the",
				"  certificate lands in `~/.cloudflared`), then create and run the tunnel.",
				"",
				"**Every one of these publishes to the public internet with no authentication",
				"in front of it**, quick tunnels included — anyone with the URL reaches the",
				"port. Do not start a tunnel unless you were explicitly asked to.",
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
