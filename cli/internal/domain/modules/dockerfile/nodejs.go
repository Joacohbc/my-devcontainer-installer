package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// NodeCurrentLink points at the bin directory of the Node version the image was
// built with, relative to devuser's home. Neither fnm nor nvm exposes a stable
// path: fnm resolves the active version from `fnm env` and nvm keeps its
// binaries under a version-named directory, so neither can be declared in the
// image environment as it stands. The build therefore asks the working shell
// where `node` actually is and leaves this link behind, which is a path that
// does not change — so `docker exec <container> node` resolves without a shell.
//
// A shell still wins over it: the init script prepends the version fnm/nvm
// selects for that session, so switching versions at runtime behaves as before
// and only shell-less callers see the version the image was built with.
const NodeCurrentLink = ".node-current"

// linkNodeCurrent records the active Node bin directory. It runs inside the
// same shell that just installed and selected the version.
const linkNodeCurrent = `ln -sfn "$(dirname "$(command -v node)")" "$HOME/` + NodeCurrentLink + `"`

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
				"",
				"`~/"+NodeCurrentLink+"` points at the version this image was built with, and is",
				"what makes `node` resolve for a command that runs with no shell at all",
				"(`docker exec`). A shell overrides it with whatever "+manager+" selects, so",
				"switching versions still works normally.",
			),
		}
	},
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		env := ContainerEnv{PathEntries: []PathEntry{"$HOME/" + NodeCurrentLink}}
		if types.StringOpt(opts, "manager", "fnm") == "fnm" {
			// nvm is a shell function, so it has no binary to put on PATH.
			env.PathEntries = append(env.PathEntries, "$HOME/.fnm")
		}
		return env
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
    su - devuser -c 'export NVM_DIR="$HOME/.nvm" && . "$NVM_DIR/nvm.sh" && %s && %s'
%s
`, installNode, linkNodeCurrent, emitShellInit(".nodejs_init.sh", initLines))
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
    su - devuser -c 'export PATH="$HOME/.fnm:$PATH" && eval "$(fnm env)" && %s && %s && %s'
%s
`, installNode, useLine, linkNodeCurrent, emitShellInit(".nodejs_init.sh", initLines))
}
