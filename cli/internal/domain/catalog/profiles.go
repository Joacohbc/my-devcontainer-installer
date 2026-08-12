package catalog

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// Repo-shipped profiles that carry scripts live here as data, next to the files
// they reference — go:embed cannot reach out of its own directory, so a profile
// with scripts cannot be a Go literal like the plain bundles below.
//
//go:embed profiles
var builtinProfileFS embed.FS

// BuiltinProfileFS exposes that tree so a built-in profile's scripts can be read
// the same way a user profile's are read off disk.
var BuiltinProfileFS fs.FS = builtinProfileFS

// BuiltinProfileRoot is the directory inside BuiltinProfileFS holding one
// subdirectory per repo-shipped profile.
const BuiltinProfileRoot = "profiles"

// ProfileFileName is the manifest inside a directory-shaped profile. A profile
// that carries custom scripts is a directory (the scripts live next to the
// manifest); one that is only a module bundle may stay a single <id>.yml file.
const ProfileFileName = "profile.yml"

// Profile is a reusable bundle of Dockerfile module ids plus, optionally, the
// user's own scripts. Older files may still carry `services:`/`mode:` keys;
// those are ignored on load.
type Profile struct {
	ID      string               `yaml:"id"`
	Label   string               `yaml:"label,omitempty"`
	Modules []string             `yaml:"modules,omitempty"`
	Scripts []types.CustomScript `yaml:"scripts,omitempty"`
	Source  string               `yaml:"-"`
	// Dir is the directory the profile's scripts are resolved against: the
	// profile's own directory for a directory-shaped profile, the containing
	// directory for a flat <id>.yml, and the path inside BuiltinProfileFS for a
	// repo-shipped one. Empty only for a profile that ships no scripts.
	Dir string `yaml:"-"`
	// Embedded marks Dir as a path inside BuiltinProfileFS rather than on the
	// host filesystem.
	Embedded bool `yaml:"-"`
}

// BuiltinProfiles is every profile the CLI ships: the plain module bundles
// below, followed by the repo-shipped ones that carry scripts (read from
// BuiltinProfileFS). A bundle that needs no files stays a Go literal, which is
// what keeps the ids the CI variant matrix depends on in one readable table.
var BuiltinProfiles = slices.Concat(plainBuiltinProfiles, embeddedProfiles())

// github-cli is listed first in every profile so its Dockerfile layer is shared
// with the base cache image (devcontainer-base), maximising Docker layer cache
// hits across all variant builds in CI. Zellij is no longer listed: it ships in
// the base image by default (always-on), so every profile gets it implicitly.
var plainBuiltinProfiles = []Profile{
	{
		ID:    "base",
		Label: "Base (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
		},
	},
	{
		ID:    "nodejs",
		Label: "Node.js (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
		},
	},
	{
		ID:    "bun",
		Label: "Bun (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
		},
	},
	{
		ID:    "java-temurin",
		Label: "Java Temurin (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:    "python",
		Label: "Python (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModulePython),
		},
	},
	{
		ID:    "go",
		Label: "Go (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "node-go",
		Label: "Node.js + Go (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "node-python",
		Label: "Node.js + Python (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModulePython),
		},
	},
	{
		ID:    "node-java-temurin",
		Label: "Node.js + Java Temurin (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:    "bun-go",
		Label: "Bun + Go (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModuleGolang),
		},
	},
	{
		ID:    "bun-python",
		Label: "Bun + Python (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModulePython),
		},
	},
	{
		ID:    "bun-java-temurin",
		Label: "Bun + Java Temurin (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModuleJavaTemurin),
		},
	},
}

// embeddedProfiles reads the repo-shipped profiles out of BuiltinProfileFS. A
// malformed one is a build-time mistake in this repo, not user input, so
// TestEmbeddedProfilesAreWellFormed asserts every entry parses instead of
// letting one disappear silently here.
func embeddedProfiles() []Profile {
	entries, err := fs.ReadDir(builtinProfileFS, BuiltinProfileRoot)
	if err != nil {
		return nil
	}
	var out []Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := path.Join(BuiltinProfileRoot, e.Name())
		data, rerr := fs.ReadFile(builtinProfileFS, path.Join(dir, ProfileFileName))
		if rerr != nil {
			continue
		}
		var p Profile
		if yaml.Unmarshal(data, &p) != nil || p.ID == "" {
			continue
		}
		p.Dir, p.Embedded = dir, true
		out = append(out, p)
	}
	return out
}

// Resolve looks an id up across the given user directories (earlier ones win)
// and then the built-ins. Passing several directories is how the legacy
// `presets/` directory keeps working alongside `profiles/`.
func Resolve(id string, userDirs ...string) (Profile, bool) {
	for _, p := range LoadUserProfiles(userDirs...) {
		if p.ID == id {
			p.Source = "user"
			return p, true
		}
	}
	for _, p := range BuiltinProfiles {
		if p.ID == id {
			p.Source = "builtin"
			return p, true
		}
	}
	return Profile{}, false
}

// All returns every user profile found in userDirs followed by the built-ins a
// user profile has not shadowed.
func All(userDirs ...string) []Profile {
	seen := map[string]bool{}
	var out []Profile
	for _, p := range LoadUserProfiles(userDirs...) {
		seen[p.ID] = true
		p.Source = "user"
		out = append(out, p)
	}
	for _, p := range BuiltinProfiles {
		if seen[p.ID] {
			continue
		}
		p.Source = "builtin"
		out = append(out, p)
	}
	return out
}

// LoadUserProfiles reads every profile in the given directories, in order, and
// drops any whose id an earlier directory already provided. Both on-disk shapes
// are accepted: a flat <id>.yml, and a <id>/profile.yml directory whose custom
// scripts sit next to the manifest.
func LoadUserProfiles(dirs ...string) []Profile {
	seen := map[string]bool{}
	var out []Profile
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, p := range loadProfileDir(dir) {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	return out
}

func loadProfileDir(dir string) []Profile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Profile
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		scriptDir := dir
		if e.IsDir() {
			// A directory-shaped profile: the manifest is inside, and so are the
			// scripts it references.
			manifest, ok := profileManifest(path)
			if !ok {
				continue
			}
			path, scriptDir = manifest, path
		} else if !hasYAMLSuffix(e.Name()) {
			continue
		}
		p, ok := readProfile(path)
		if !ok {
			continue
		}
		p.Dir = scriptDir
		out = append(out, p)
	}
	return out
}

// profileManifest returns the manifest path inside a directory-shaped profile.
func profileManifest(dir string) (string, bool) {
	for _, name := range []string{ProfileFileName, "profile.yaml"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func readProfile(path string) (Profile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, false
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Profile{}, false
	}
	if p.ID == "" {
		return Profile{}, false
	}
	return p, true
}

func hasYAMLSuffix(name string) bool {
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}
