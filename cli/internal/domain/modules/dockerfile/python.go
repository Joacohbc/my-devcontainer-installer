package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PythonModule = &ModuleSpec{
	ID:         "python",
	Label:      "Python (python3 + pip, optional uv)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Options: []types.ModuleOption{
		{
			ID:      "uv",
			Label:   "Install uv (Astral, for devuser)",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		uv := true
		if v, ok := opts["uv"]; ok {
			if b, ok := v.(bool); ok {
				uv = b
			}
		}
		uvBlock := ""
		if uv {
			uvBlock = fmt.Sprintf(`
# Install uv (Astral Python installer/manager) for devuser
RUN su - devuser -c 'curl -LsSf https://astral.sh/uv/install.sh | sh'
%s
`, emitShellInit(".python_init.sh", []string{`export PATH="$HOME/.local/bin:$PATH"`}))
		}
		return fmt.Sprintf(`##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip
%s`, uvBlock)
	},
}
