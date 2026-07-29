package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// BakedAliasFile is the image-baked default alias/function file, sourced from
// every rc file. It is owned by the CLI and rewritten on every build.
const BakedAliasFile = ".devcontainer_aliases.sh"

// UserAliasFile is the user's own alias file. The entrypoint symlinks it into
// the shared-config volume (types.SharedConfigEntries), so it persists across
// every container and is edited on the host with `config alias edit`. It is
// sourced AFTER BakedAliasFile so user definitions win.
const UserAliasFile = ".alias.sh"

// YoloAgentsFlagFile gates the agent aliases inside alias.sh. Its mere presence
// turns them on, which keeps the build-time difference between yoloAgents on and
// off down to a single `touch` — no templating of the shipped script.
const YoloAgentsFlagFile = ".devcontainer_agents_yolo"

// ContextScriptName is the introspection command installed on PATH.
const ContextScriptName = "get-devcontainer-context"

// AliasesModule installs the CLI's default shell aliases/functions (kill_port,
// npm→pnpm, pip→uv, the permission-prompt-free agent aliases), the ~/CONTEXT.md
// orientation document and the get-devcontainer-context introspection command.
//
// It is Always-on and sits in CategoryBase directly after BaseModule: the zsh
// installer inside BaseModule rewrites ~/.zshrc from scratch, so anything that
// appends to the rc files must run after it.
var AliasesModule = &ModuleSpec{
	ID:        types.ModuleAliases,
	Label:     "Aliases & container context (kill_port, npm→pnpm, pip→uv, ~/CONTEXT.md)",
	Category:  types.CategoryBase,
	Always:    true,
	CopyFiles: []string{"alias.sh", "setup-context.sh", ContextScriptName + ".sh"},
	Options: []types.ModuleOption{
		{
			ID:      "yoloAgents",
			Label:   "Run AI agents without permission prompts (the container is the sandbox)",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		// The yoloAgents difference is a single `touch` run as devuser inside the
		// same login shell as the context installer: alias.sh keys off the flag
		// file's existence, so the shipped script never has to be templated.
		devuserStep := "/tmp/setup-context.sh"
		if types.BoolOpt(opts, "yoloAgents", true) {
			devuserStep += ` && touch \$HOME/` + YoloAgentsFlagFile
		}
		return fmt.Sprintf(`##
## SHELL ALIASES & CONTAINER CONTEXT
##
# Default aliases/functions (kill_port, npm->pnpm, pip->uv, prompt-free agents),
# the ~/CONTEXT.md orientation document for AI agents and the
# get-devcontainer-context introspection command. Installed in a single layer;
# the installer script is removed in that same layer.
COPY alias.sh %[1]s/%[2]s
COPY %[3]s.sh %[1]s/.local/bin/%[3]s
COPY setup-context.sh /tmp/setup-context.sh
RUN chmod 0644 %[1]s/%[2]s && \
    chmod 0755 %[1]s/.local/bin/%[3]s && \
    chmod +x /tmp/setup-context.sh && \
    chown -R devuser:devuser %[1]s/%[2]s %[1]s/.local && \
    su - devuser -c "%[4]s" && \
    rm /tmp/setup-context.sh
%[5]s
`,
			devuserHome, BakedAliasFile, ContextScriptName, devuserStep,
			emitShellSources(
				shellSource{File: BakedAliasFile},
				// The user's own file is sourced last so it overrides the defaults,
				// and guarded because it only exists when the shared-config volume
				// is mounted.
				shellSource{File: UserAliasFile, Guarded: true},
			),
		)
	},
}
