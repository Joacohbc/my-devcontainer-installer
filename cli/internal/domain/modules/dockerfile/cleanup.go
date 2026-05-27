package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var CleanupModule = &ModuleSpec{
	ID:        "cleanup",
	Label:     "Cleanup + entrypoint + EXPOSE 22",
	Category:  types.CategoryCleanup,
	Always:    true,
	CopyFiles: []string{"entrypoint.sh"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"login-github-cli.sh"}
	},
	Render: func(opts map[string]any) string {
		return `##
## CLEANUP & ENTRYPOINT
##
RUN apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/* && \
    rm -rf /var/tmp/*

RUN mkdir -p /var/run/sshd && \
    chmod 755 /var/run/sshd

EXPOSE 22

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
`
	},
}
