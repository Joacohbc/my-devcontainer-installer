package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var GolangModule = &ModuleSpec{
	ID:        "go",
	Label:     "Go / Golang (Latest version)",
	Category:  types.CategoryLang,
	CopyFiles: []string{"golang_utils.sh"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"update_golang.sh", "golang_utils.sh"}
	},
	Render: func(opts map[string]any) string {
		return `##
## GO
##
COPY golang_utils.sh /tmp/golang_utils.sh
RUN bash -c "source /tmp/golang_utils.sh && install_golang" && rm /tmp/golang_utils.sh
`
	},
}
