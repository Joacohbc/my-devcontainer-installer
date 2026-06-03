package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CavemanModule = &ModuleSpec{
	ID:         types.ModuleCaveman,
	Label:      "Caveman (AI agent output compression + hooks)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryAITools,

	// The caveman installer is Node-based (bin/install.js) and needs Node >= 18.
	Requires: []types.ModuleID{types.ModuleNodejs},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"install-caveman.sh"}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf("##\n## Caveman — install script shipped under %s\n##\n", types.PostScriptDir)
	},
}
