package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var NgrokModule = &ModuleSpec{
	ID:         types.ModuleNgrok,
	Label:      "ngrok",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "ngrok",
			Body: ctxBody(
				"`ngrok` is installed for exposing a local port on a public URL. It needs an",
				"auth token (`ngrok config add-authtoken …`) before it will run.",
				"",
				"**It publishes to the public internet.** Do not start a tunnel unless you",
				"were explicitly asked to.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## NGROK
##
RUN wget -nv -O /tmp/ngrok.asc https://ngrok-agent.s3.amazonaws.com/ngrok.asc && \
    cat /tmp/ngrok.asc | tee /etc/apt/trusted.gpg.d/ngrok.asc > /dev/null && \
    echo "deb https://ngrok-agent.s3.amazonaws.com bookworm main" | tee /etc/apt/sources.list.d/ngrok.list > /dev/null && \
    apt-get update && \
    apt-get install -y ngrok && \
    %s
`, aptCleanup())
	},
}
