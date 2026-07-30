package dockerfile

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ctxBody joins the markdown lines of a ContextSection body.
//
// Section bodies are written as double-quoted Go strings rather than raw
// backtick literals on purpose: markdown code spans are backticked, and a raw
// literal cannot contain a backtick. Joining lines keeps the source readable
// without escaping gymnastics.
func ctxBody(lines ...string) string {
	return strings.Join(lines, "\n")
}

// agentCtx builds the ~/CONTEXT.md entry for an AI agent CLI. They all share the
// same shape — installed by a post-script rather than baked into the image, with
// config persisted in the shared volume — so only the tool-specific lines differ.
//
// autoStart mirrors ModuleSpec.PostScriptAutoStart: an auto-started installer
// runs in the background on first boot (so the binary may not exist for the
// first few seconds), while a manual one has to be invoked by hand.
func agentCtx(title, command string, autoStart bool, extra ...string) *types.ContextSection {
	lines := []string{command}
	if autoStart {
		lines = append(lines,
			"",
			"Installed by a post-script that the entrypoint runs in the background the",
			"first time the container starts, so it may be unavailable for a few seconds",
			"after boot. Check `~/.post-script-state/` for the install log if it is missing.",
		)
	}
	if len(extra) > 0 {
		lines = append(lines, "")
		lines = append(lines, extra...)
	}
	return &types.ContextSection{Title: title, Body: ctxBody(lines...)}
}
