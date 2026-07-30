package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var NodejsModule = &ModuleSpec{
	ID:         types.ModuleNodejs,
	Label:      "Node.js (fnm or nvm, for devuser)",
	Category:   types.CategoryRuntime,
	UICategory: types.UICategoryLanguages,
	Requires:   []types.ModuleID{types.ModuleGithubCli},
	Options: []types.ModuleOption{
		{
			ID:    "manager",
			Label: "Node.js Version manager",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "fnm", Label: "fnm (Fast Node Manager, Rust)"},
				{Value: "nvm", Label: "nvm (Node Version Manager)"},
			},
			Default: "fnm",
		},
		{
			ID:    "version",
			Label: "Node version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "lts", Label: "LTS (auto)"},
				{Value: "22", Label: "Node 22 (Maintenance LTS)"},
				{Value: "24", Label: "Node 24 (Active LTS)"},
			},
			Default: "lts",
		},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		manager := types.StringOpt(opts, "manager", "fnm")
		version := types.StringOpt(opts, "version", "lts")
		versionLabel := "Node " + version
		if version == "lts" {
			versionLabel = "the latest LTS"
		}
		return &types.ContextSection{
			Title: "Node.js",
			Body: ctxBody(
				"Installed for `devuser` through **"+manager+"**, pinned to "+versionLabel+".",
				"Run `node --version` to see the exact build.",
				"",
				"Switch versions with `"+manager+" use <version>` — never with `sudo apt`. The",
				"version manager is initialised from `~/.nodejs_init.sh`, which every shell",
				"sources, so `node` is on PATH in login and non-login shells alike.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		manager := types.StringOpt(opts, "manager", "fnm")
		version := types.StringOpt(opts, "version", "lts")
		if manager == "nvm" {
			return renderNvm(version)
		}
		return renderFnm(version)
	},
}

func renderNvm(version string) string {
	var useLine, installNode string
	if version == "lts" {
		useLine = "nvm use --lts > /dev/null"
		installNode = "nvm install --lts"
	} else {
		useLine = fmt.Sprintf("nvm use %s > /dev/null", version)
		installNode = fmt.Sprintf("nvm install %s", version)
	}
	initLines := []string{
		`export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"`,
		`[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"`,
		useLine,
	}
	return fmt.Sprintf(`##
## NVM + NODE (devuser)
##
RUN NVM_VERSION=$(curl -s https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .tag_name) && \
    su - devuser -c "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION}/install.sh | bash" && \
    su - devuser -c 'export NVM_DIR="$HOME/.nvm" && . "$NVM_DIR/nvm.sh" && %s'
%s
`, installNode, emitShellInit(".nodejs_init.sh", initLines))
}

func renderFnm(version string) string {
	var useLine, installNode string
	if version == "lts" {
		useLine = "fnm use lts-latest > /dev/null"
		installNode = "fnm install --lts"
	} else {
		useLine = fmt.Sprintf("fnm use %s > /dev/null", version)
		installNode = fmt.Sprintf("fnm install %s", version)
	}
	initLines := []string{
		`export PATH="$HOME/.fnm:$PATH"`,
		`eval "$(fnm env --use-on-cd)"`,
		useLine,
	}
	return fmt.Sprintf(`##
## FNM + NODE (devuser)
##
RUN su - devuser -c 'curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$HOME/.fnm" --skip-shell' && \
    su - devuser -c 'export PATH="$HOME/.fnm:$PATH" && eval "$(fnm env)" && %s'
%s
`, installNode, emitShellInit(".nodejs_init.sh", initLines))
}
