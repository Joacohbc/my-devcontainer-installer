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
	{Name: "entrypoint", File: "entrypoint.sh", Kind: KindBuild, Label: "Container entrypoint"},
	{Name: "golang-utils", File: "golang_utils.sh", Kind: KindBuild, Label: "Go install/update utilities"},
	{Name: "install-antigravity", File: "install-antigravity.sh", Kind: KindScript, Label: "Antigravity CLI installer"},
	{Name: "install-claude-code", File: "install-claude-code.sh", Kind: KindScript, Label: "Claude Code installer"},
	{Name: "install-codex-cli", File: "install-codex-cli.sh", Kind: KindScript, Label: "Codex CLI installer"},
	{Name: "install-copilot", File: "install-copilot.sh", Kind: KindScript, Label: "GitHub Copilot CLI installer"},
	{Name: "install-opencode", File: "install-opencode.sh", Kind: KindScript, Label: "OpenCode installer"},
	{Name: "login-github-cli", File: "login-github-cli.sh", Kind: KindScript, Label: "GitHub CLI login helper"},
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
