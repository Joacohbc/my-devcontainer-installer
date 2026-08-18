package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// CleanupModule ships the entrypoint and everything the entrypoint needs before
// it can do anything: the shared-config shell library and the rendered
// catalogue that library reads.
//
// Only the library is in CopyFiles. The catalogue is GENERATED from
// types.SharedConfigEntries and written into the build dir by prepareBuildDir,
// the same way CONTEXT.md is — listing it here would send assets.Preflight
// looking for it in the embedded FS and report it missing.
var CleanupModule = &ModuleSpec{
	ID:        types.ModuleCleanup,
	Label:     "Cleanup + entrypoint + EXPOSE 22",
	Category:  types.CategoryCleanup,
	Always:    true,
	CopyFiles: []string{"entrypoint.sh", types.SharedConfigLibFile},
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

COPY %s %s/%s
COPY %s %s/%s

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
`, aptCleanup(),
			types.SharedConfigLibFile, types.SharedConfigLibDir, types.SharedConfigLibFile,
			types.SharedConfigTableFileName, types.SharedConfigLibDir, types.SharedConfigTableFileName)
	},
}
