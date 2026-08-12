package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

type ModuleSpec struct {
	ID         types.ModuleID
	Label      string
	Category   types.DockerfileCategory
	UICategory types.UICategory
	Always     bool
	// Internal marks a module the generator derives rather than the user picks:
	// it never appears in the wizard's categories and therefore declares no
	// UICategory. Mirrors compose.ServiceSpec.Internal.
	Internal  bool
	Requires  []types.ModuleID
	Conflicts []types.ModuleID
	Options   []types.ModuleOption
	// RequiresEnv are env vars this module's tool reads at runtime. The generate
	// wizard prompts for each one, the answer lands in the project's .env, and
	// the devcontainer service passes it into the container. An empty answer is a
	// supported state, not a missing one: the var stays out of the .env and
	// arrives empty, so the module must work without it.
	RequiresEnv []types.RequiredEnvVar
	// ProvidesEnv is the environment this module adds to the image: the PATH
	// entries its tools live in and the variables they read. It is the
	// counterpart of RequiresEnv — that one is asked of the user at runtime,
	// this one the module knows at build time.
	//
	// Declaring it as data instead of writing `export …` into a shell-init file
	// is what lets the generator render it to every target: the Dockerfile's ENV
	// (inherited by `docker exec`, which starts no shell) and a sourced script
	// (read by shells that never see ENV). Aliases and functions do not belong
	// here — a process cannot inherit them; they stay in alias.sh.
	//
	// Nil means the module adds nothing. Only genuinely dynamic initialisation
	// (`eval "$(fnm env)"`) still belongs in Render's shell init.
	ProvidesEnv     func(opts map[string]any) ContainerEnv
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
