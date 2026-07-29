package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var GolangModule = &ModuleSpec{
	ID:         types.ModuleGolang,
	Label:      "Go / Golang (Latest version)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	CopyFiles:  []string{"golang_utils.sh"},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"update_golang.sh", "golang_utils.sh"}
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Go",
			Body: ctxBody(
				"The latest stable Go toolchain is installed system-wide (`go version` for the",
				"exact build). `go install` puts binaries in `~/go/bin`.",
				"",
				"Upgrade it in place with `~/post-script/update_golang.sh` rather than",
				"`apt`, which has no Go package new enough to matter.",
			),
		}
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
