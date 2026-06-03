package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var GraphifyModule = &ModuleSpec{
	ID:         types.ModuleGraphify,
	Label:      "Graphify (graphifyy — codebase knowledge graphs)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,
	// graphifyy is a Python 3.10+ package (installed via uv/pipx/pip).
	Requires: []types.ModuleID{types.ModulePython},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-graphify.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Graphify — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
