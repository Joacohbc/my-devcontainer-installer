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
