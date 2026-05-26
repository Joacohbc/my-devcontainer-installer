package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"

var GolangModule = &DockerfileModuleSpec{
	ID:        "go",
	Label:     "Go / Golang (Latest version)",
	Category:  core.CategoryLang,
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
