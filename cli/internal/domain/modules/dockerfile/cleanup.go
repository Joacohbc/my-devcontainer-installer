package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CleanupModule = &ModuleSpec{
	ID:        types.ModuleCleanup,
	Label:     "Cleanup + entrypoint + EXPOSE 22",
	Category:  types.CategoryCleanup,
	Always:    true,
	CopyFiles: []string{"entrypoint.sh"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"login-github-cli.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## CLEANUP & ENTRYPOINT
##
RUN %s

RUN mkdir -p /var/run/sshd && \
    chmod 755 /var/run/sshd

EXPOSE 22

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
`, aptCleanup())
	},
}
