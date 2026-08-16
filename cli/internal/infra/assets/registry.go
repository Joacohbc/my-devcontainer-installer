package assets

// AssetKind classifies an embedded asset by how it can be used.
type AssetKind string

// KindScript marks a shell script that can be copied into a running container
// at runtime (e.g. AI CLI installers and helper scripts). Such scripts are
// selectable by the `copy --asset` command.
const KindScript AssetKind = "script"

// KindBuild marks a script that is only meaningful during the image build
// (entrypoint, language installers invoked from the Dockerfile). These are not
// reusable at runtime and are therefore excluded from the copyable set.
const KindBuild AssetKind = "build"

// KindDoc marks a markdown document baked into the image at build time (the
// global agent skill). Like KindBuild it is not runtime-copyable: dropping it
// into a running container would leave a stale copy the next rebuild ignores.
const KindDoc AssetKind = "doc"

// Asset describes an embedded asset and its capabilities.
type Asset struct {
	Name  string // selection/completion id, e.g. "install-claude-code"
	File  string // embedded filename, e.g. "install-claude-code.sh"
	Kind  AssetKind
	Label string
}

// Registry lists every embedded asset with its metadata. Scripts marked
// KindScript are runtime-copyable via `copy --asset`; KindBuild scripts are
// build-time only and never offered for copying.
var Registry = []Asset{
	{Name: "alias", File: "alias.sh", Kind: KindBuild, Label: "Default shell aliases and helpers"},
	{Name: "entrypoint", File: "entrypoint.sh", Kind: KindBuild, Label: "Container entrypoint"},
	{Name: "get-devcontainer-context", File: "get-devcontainer-context.sh", Kind: KindScript, Label: "Container context / installed-tool report"},
	{Name: "golang-utils", File: "golang_utils.sh", Kind: KindBuild, Label: "Go install/update utilities"},
	{Name: "install-antigravity", File: "install-antigravity.sh", Kind: KindScript, Label: "Antigravity CLI installer"},
	{Name: "install-caveman", File: "install-caveman.sh", Kind: KindScript, Label: "Caveman installer"},
	{Name: "install-claude-code", File: "install-claude-code.sh", Kind: KindScript, Label: "Claude Code installer"},
	{Name: "install-claude-mem", File: "install-claude-mem.sh", Kind: KindScript, Label: "Claude-Mem installer"},
	{Name: "install-codex-cli", File: "install-codex-cli.sh", Kind: KindScript, Label: "Codex CLI installer"},
	{Name: "install-context-mode", File: "install-context-mode.sh", Kind: KindScript, Label: "Context Mode installer"},
	{Name: "install-copilot", File: "install-copilot.sh", Kind: KindScript, Label: "GitHub Copilot CLI installer"},
	{Name: "install-graphify", File: "install-graphify.sh", Kind: KindScript, Label: "Graphify installer"},
	{Name: "install-opencode", File: "install-opencode.sh", Kind: KindScript, Label: "OpenCode installer"},
	{Name: "login-github-cli", File: "login-github-cli.sh", Kind: KindScript, Label: "GitHub CLI login helper"},
	{Name: "setup-help", File: "setup-help.sh", Kind: KindBuild, Label: "~/help quick reference writer"},
	{Name: "skill-devcontainer-context", File: "skill-devcontainer-context.md", Kind: KindDoc, Label: "Global agent skill: read the container context"},
	{Name: "update-golang", File: "update_golang.sh", Kind: KindBuild, Label: "Go update script"},
	{Name: "zsh-installer", File: "zsh-installer.sh", Kind: KindBuild, Label: "Zsh configuration installer"},
}

// CopyableAssets returns the assets that can be copied into a running container
// (those classified as KindScript).
func CopyableAssets() []Asset {
	out := make([]Asset, 0, len(Registry))
	for _, a := range Registry {
		if a.Kind == KindScript {
			out = append(out, a)
		}
	}
	return out
}

// CopyableNames returns the selection ids of every copyable asset, suitable for
// shell completion.
func CopyableNames() []string {
	names := make([]string, 0, len(Registry))
	for _, a := range CopyableAssets() {
		names = append(names, a.Name)
	}
	return names
}

// LookupCopyable resolves a copyable asset by its selection id (Name).
func LookupCopyable(name string) (Asset, bool) {
	for _, a := range CopyableAssets() {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ReadAsset returns the raw bytes of an embedded asset by filename.
func ReadAsset(file string) ([]byte, error) {
	return embedded.ReadFile(file)
}
