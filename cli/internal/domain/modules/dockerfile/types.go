package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

type ModuleSpec struct {
	ID              string
	Label           string
	Category        types.DockerfileCategory
	UICategory      types.UICategory
	Always          bool
	Requires        []string
	Conflicts       []string
	Options         []types.ModuleOption
	CopyFiles       []string
	PostScriptFiles func(opts map[string]any) []string
	Render          func(opts map[string]any) string
}
