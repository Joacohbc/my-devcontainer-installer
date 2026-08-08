package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// TailscaleAuthKeyEnv is the env var a Tailscale auth key is passed through. It
// keeps the name tailscaled itself uses (TS_AUTHKEY), so a key exported for the
// official container image works here unchanged.
const TailscaleAuthKeyEnv = "TS_AUTHKEY"

var TailscaleModule = &ModuleSpec{
	ID:         types.ModuleTailscale,
	Label:      "Tailscale",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: TailscaleAuthKeyEnv, Prompt: "Tailscale auth key (optional; empty = log in from the container with 'tailscale up')"},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Tailscale",
			Body: ctxBody(
				"`tailscale` and `tailscaled` are installed in this container. Joining a",
				"tailnet gives it a private WireGuard address only your own devices reach —",
				"unlike a Cloudflare tunnel or ngrok, nothing is published publicly by",
				"default.",
				"",
				"There is no systemd here, so **the daemon does not start on its own**, and",
				"the container has neither `/dev/net/tun` nor `NET_ADMIN`, so it has to run in",
				"userspace networking mode:",
				"",
				"```sh",
				"sudo tailscaled --tun=userspace-networking \\",
				"  --socks5-server=localhost:1055 \\",
				"  --outbound-http-proxy-listen=localhost:1055 >/tmp/tailscaled.log 2>&1 &",
				"```",
				"",
				"Then authenticate, in one of two ways:",
				"",
				"- **With an auth key.** If `$"+TailscaleAuthKeyEnv+"` is set,",
				"  `sudo tailscale up --authkey=\"$"+TailscaleAuthKeyEnv+"\" --hostname=<name>`",
				"  joins the tailnet with no browser and no prompt.",
				"- **Without one.** `sudo tailscale up` prints a login URL to open in a",
				"  browser on your own machine; the container waits until you approve it.",
				"",
				"Userspace mode changes how traffic flows, in both directions:",
				"",
				"- **Inbound works normally.** Other tailnet nodes reach services here on this",
				"  node's Tailscale IP; the daemon proxies those connections to `localhost`.",
				"- **Outbound must go through the proxy.** A plain `curl http://<node>` does",
				"  not route into the tailnet — send it through the SOCKS5/HTTP proxy the",
				"  daemon opened, e.g. `ALL_PROXY=socks5://localhost:1055 curl http://<node>`.",
				"- Exit nodes, subnet routes and a real `tailscale0` interface need the",
				"  container to be started with `--device /dev/net/tun --cap-add=NET_ADMIN`,",
				"  which the generated compose file does not grant.",
				"",
				"The node state lives in `/var/lib/tailscale`, i.e. in the container's",
				"writable layer, so recreating the container loses its identity and you",
				"authenticate again. An **ephemeral** auth key is the tidy way to do that —",
				"the node disappears from the admin console instead of piling up.",
				"",
				"`tailscale serve` stays inside the tailnet, but **`tailscale funnel`",
				"publishes the port to the public internet with no authentication in front of",
				"it**. Do not start a funnel unless you were explicitly asked to.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## TAILSCALE
##
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    . /etc/os-release && \
    wget -nv -O /etc/apt/keyrings/tailscale-archive-keyring.gpg "https://pkgs.tailscale.com/stable/ubuntu/${VERSION_CODENAME}.noarmor.gpg" && \
    chmod go+r /etc/apt/keyrings/tailscale-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/tailscale-archive-keyring.gpg] https://pkgs.tailscale.com/stable/ubuntu ${VERSION_CODENAME} main" | tee /etc/apt/sources.list.d/tailscale.list > /dev/null && \
    apt-get update && \
    apt-get install -y tailscale && \
    %s
`, aptCleanup())
	},
}
