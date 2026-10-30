package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

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
	// Skills are the agent skills a project starting from this profile installs,
	// and SkillsMode how they get installed. They are project-scoped, so they are
	// applied to the project rather than baked into the image.
	Skills     []types.SkillID `yaml:"skills,omitempty"`
	SkillsMode types.SkillMode `yaml:"skills_mode,omitempty"`
	// Ports are published by the generated stack, ForwardPorts are the tunnels
	// `port-forward` opens with no argument. Both accept the same two shapes: a
	// bare container port ("3000"), which lets Docker pick a free host port so
	// two projects from this profile can run at once, or an explicit mapping
	// ("3000:3000"), which is predictable but collides on the second project.
	Ports        []string `yaml:"ports,omitempty"`
	ForwardPorts []string `yaml:"forward_ports,omitempty"`
	// Remote marks that this profile id also has a published
	// ghcr.io/devcontainer-<id> image, built by CI from this exact module list —
	// so it is safe to offer for --profile under mode=profiles (a pull instead of a
	// local build). A profile without it can still be applied with --profile in
	// mode=custom; it is just never a pull target.
	Remote bool   `yaml:"remote,omitempty"`
	Source string `yaml:"-"`
	// Dir is the directory the profile's scripts are resolved against: the
	// profile's own directory for a directory-shaped profile, and the containing
	// directory for a flat <id>.yml. Empty for a profile that ships no scripts.
	Dir string `yaml:"-"`
}

// BuiltinProfiles is every profile the CLI ships: the plain module bundles below.
var BuiltinProfiles = plainBuiltinProfiles

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
		ID:     "nodejs",
		Remote: true,
		Label:  "Node.js (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
		},
	},
	{
		ID:     "bun",
		Remote: true,
		Label:  "Bun (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
		},
	},
	{
		ID:     "java-temurin",
		Remote: true,
		Label:  "Java Temurin (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:     "python",
		Remote: true,
		Label:  "Python (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModulePython),
		},
	},
	{
		ID:     "go",
		Remote: true,
		Label:  "Go (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleGolang),
		},
	},
	{
		ID:     "node-go",
		Remote: true,
		Label:  "Node.js + Go (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleGolang),
		},
	},
	{
		ID:     "node-python",
		Remote: true,
		Label:  "Node.js + Python (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModulePython),
		},
	},
	{
		ID:     "node-java-temurin",
		Remote: true,
		Label:  "Node.js + Java Temurin (pnpm, GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleNodejs),
			string(types.ModulePnpm),
			string(types.ModuleJavaTemurin),
		},
	},
	{
		ID:     "bun-go",
		Remote: true,
		Label:  "Bun + Go (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModuleGolang),
		},
	},
	{
		ID:     "bun-python",
		Remote: true,
		Label:  "Bun + Python (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModulePython),
		},
	},
	{
		ID:     "bun-java-temurin",
		Remote: true,
		Label:  "Bun + Java Temurin (GitHub CLI)",
		Modules: []string{
			string(types.ModuleGithubCli),
			string(types.ModuleBun),
			string(types.ModuleJavaTemurin),
		},
	},
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

func appendUnique[T comparable](dst []T, seen map[T]bool, items []T) []T {
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			dst = append(dst, item)
		}
	}
	return dst
}

// MergeProfiles merges an ordered list of profiles into a single effective profile.
//   - Modules: union of module IDs, deduplicated, preserving first-seen order.
//   - Scripts: concatenated in order. If two scripts share the same destination
//     build file name (s.BuildFile()) across different profiles, an error is returned
//     naming both profiles. If a script's Source is not yet set and the originating
//     profile has a Dir, Source is resolved against it.
//   - Skills: union of skill IDs, deduplicated, preserving first-seen order.
//   - Ports and ForwardPorts: union, deduplicated, preserving first-seen order.
//   - SkillsMode: if multiple profiles declare conflicting explicit skills modes, an
//     error is returned. Otherwise, the explicit mode (if any) is used.
func MergeProfiles(profiles ...Profile) (Profile, error) {
	if len(profiles) == 0 {
		return Profile{}, nil
	}
	if len(profiles) == 1 {
		return profiles[0], nil
	}

	var (
		ids          []string
		labels       []string
		modules      []string
		scripts      []types.CustomScript
		skills       []types.SkillID
		ports        []string
		forwardPorts []string

		seenModules      = make(map[string]bool)
		seenSkills       = make(map[types.SkillID]bool)
		seenPorts        = make(map[string]bool)
		seenForwardPorts = make(map[string]bool)
		scriptDests      = make(map[string]string) // dest build file -> profile ID/label

		explicitMode     types.SkillMode
		explicitModeProf string
	)

	for _, p := range profiles {
		profName := p.ID
		if profName == "" {
			profName = p.Label
		}
		if profName == "" {
			profName = "unnamed"
		}

		if p.ID != "" {
			ids = append(ids, p.ID)
		}
		if p.Label != "" {
			labels = append(labels, p.Label)
		}

		modules = appendUnique(modules, seenModules, p.Modules)

		for _, s := range p.Scripts {
			dest := s.BuildFile()
			if prevProf, exists := scriptDests[dest]; exists && prevProf != profName {
				return Profile{}, fmt.Errorf("script collision: profiles %q and %q both declare destination %q", prevProf, profName, dest)
			}
			scriptDests[dest] = profName

			if s.Source == "" && p.Dir != "" {
				s.Source = filepath.Join(p.Dir, s.File)
			}
			scripts = append(scripts, s)
		}

		skills = appendUnique(skills, seenSkills, p.Skills)
		ports = appendUnique(ports, seenPorts, p.Ports)
		forwardPorts = appendUnique(forwardPorts, seenForwardPorts, p.ForwardPorts)

		if p.SkillsMode != "" {
			if explicitMode == "" {
				explicitMode = p.SkillsMode
				explicitModeProf = profName
			} else if explicitMode != p.SkillsMode {
				return Profile{}, fmt.Errorf("conflicting skills modes: profile %q specifies %q, but profile %q specifies %q", explicitModeProf, explicitMode, profName, p.SkillsMode)
			}
		}
	}

	merged := Profile{
		ID:           strings.Join(ids, ","),
		Label:        strings.Join(labels, " + "),
		Modules:      modules,
		Scripts:      scripts,
		Skills:       skills,
		SkillsMode:   explicitMode,
		Ports:        ports,
		ForwardPorts: forwardPorts,
		Remote:       false,
	}
	return merged, nil
}

// ResolveComposed resolves each ID in the given list and merges them into an
// effective profile. If any profile ID is unknown, an error is returned.
func ResolveComposed(ids []string, userDirs ...string) (Profile, error) {
	if len(ids) == 0 {
		return Profile{}, nil
	}
	resolved := make([]Profile, 0, len(ids))
	for _, id := range ids {
		p, ok := Resolve(id, userDirs...)
		if !ok {
			return Profile{}, fmt.Errorf("unknown profile: %s", id)
		}
		resolved = append(resolved, p)
	}
	return MergeProfiles(resolved...)
}
