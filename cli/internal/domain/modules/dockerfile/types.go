package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

type ModuleSpec struct {
	ID              types.ModuleID
	Label           string
	Category        types.DockerfileCategory
	UICategory      types.UICategory
	Always          bool
	Requires        []types.ModuleID
	Conflicts       []types.ModuleID
	Options         []types.ModuleOption
	CopyFiles       []string
	PostScriptFiles func(opts map[string]any) []string
	// PostScriptAutoStart marks this module's PostScriptFiles as non-interactive
	// installers that the container entrypoint runs automatically (in the
	// background, once per container) on start, instead of leaving them as
	// manual scripts under PostScriptDir. Interactive scripts (codex, github
	// login) must leave this false.
	PostScriptAutoStart bool
	// PostScriptStartOrder controls the run order of auto-start scripts (lower
	// runs first). Modules that wire themselves into other already-installed
	// agents (graphify, caveman) use a high value so they run last. Zero falls
	// back to the default order.
	PostScriptStartOrder int
	Render               func(opts map[string]any) string
	// Context is this module's entry in the generated ~/CONTEXT.md: what it puts
	// in the container and how an AI agent is expected to use it. Nil means the
	// module adds nothing an agent needs to know (pure build plumbing). Returning
	// nil for a given opts is also allowed, so a module can stay silent when the
	// option that made it interesting is off.
	//
	// It is rendered at build time from the resolved options, so anything it
	// mentions must be the value actually baked into this image.
	Context func(opts map[string]any) *types.ContextSection
}
