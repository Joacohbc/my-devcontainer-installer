package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/core"

type DockerfileModuleSpec struct {
	ID              string
	Label           string
	Category        core.DockerfileCategory
	Always          bool
	Requires        []string
	Conflicts       []string
	Options         []core.ModuleOption
	CopyFiles       []string
	PostScriptFiles func(opts map[string]any) []string
	Render          func(opts map[string]any) string
}
