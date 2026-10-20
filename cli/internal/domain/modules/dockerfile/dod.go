package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var DodModule = &ModuleSpec{
	ID:         types.ModuleDod,
	Label:      "Docker CLI + Socket (DoD)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Docker (docker-outside-of-docker)",
			Body: ctxBody(
				"The `docker` CLI is installed and `/var/run/docker.sock` is bind-mounted from",
				"the host, so `docker` commands here drive the **host's** daemon — not a nested",
				"one. Containers you start are siblings of this one, not children.",
				"",
				"Consequences worth remembering: a bind mount path you pass to `docker run` is",
				"resolved on the *host* filesystem, not this container's, and anything you",
				"create outlives this container. Clean up after yourself.",
				"",
				"**This container is not a sandbox while the socket is mounted.** Access to",
				"the host daemon is root on the host: `docker run -v /:/host …` reads and",
				"writes the whole machine. Never run an agent with its approvals disabled",
				"(`claude_yolo`, `codex_yolo`, …) on the assumption that the container",
				"contains it, and ask before any `docker` command that is not about this",
				"project.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## DOCKER CLI (DoD)
##
RUN apt-get update && \
    apt-get install -y ca-certificates curl gnupg && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    chmod a+r /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu jammy stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    %s
`, aptCleanup())
	},
}
