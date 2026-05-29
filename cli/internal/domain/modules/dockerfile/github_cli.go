package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

var GithubCliModule = &ModuleSpec{
	ID:         "github-cli",
	Label:      "GitHub CLI (gh)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return `##
## GITHUB CLI
##
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    wget -nv -O /tmp/githubcli-archive-keyring.gpg https://cli.github.com/packages/githubcli-archive-keyring.gpg && \
    cat /tmp/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get update && \
    apt-get install -y gh && \
    apt-get clean
`
	},
}
