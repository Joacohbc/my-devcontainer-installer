package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CloudflaredModule = &ModuleSpec{
	ID:         types.ModuleCloudflared,
	Label:      "Cloudflare Tunnel (cloudflared)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
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
				"  immediately. Nothing to configure. The URL is ephemeral: it dies with the",
				"  process and is different every run, so it is the right choice for a one-off",
				"  demo or webhook test, not for a stable address.",
				"- **Named tunnel with a connector token.**",
				"  `cloudflared tunnel run --token <token>` runs a tunnel that was created in",
				"  the Cloudflare dashboard, where its routing is configured too. The CLI does",
				"  not ask for that token and does not pass one in, so it has to be given here",
				"  (or exported as `$TUNNEL_TOKEN`, which is what `cloudflared` reads when the",
				"  flag is omitted).",
				"- **Named tunnel without a token.** Run `cloudflared tunnel login` first (the",
				"  certificate lands in `~/.cloudflared`), then create and run the tunnel. This",
				"  is also what the account-level commands need — `cloudflared tunnel route ip",
				"  add <CIDR> <tunnel>` and the rest of `tunnel route`, which a connector token",
				"  does not authorize.",
				"",
				"**Every one of these publishes to the public internet with no authentication",
				"in front of it**, quick tunnels included — anyone with the URL reaches the",
				"port. A tunnel whose only route is a private network is the exception, and it",
				"still needs the Zero Trust side configured. Do not start a tunnel unless you",
				"were explicitly asked to.",
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
