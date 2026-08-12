package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ProfileDirName is where user profiles are written and read from.
const ProfileDirName = "profiles"

// customScriptFileRe is the accepted shape of a custom script's file name.
var customScriptFileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.sh$`)

// LegacyProfileDirName is the directory profiles were called presets in. It is
// still read so an existing installation keeps working, but nothing is written
// there anymore.
const LegacyProfileDirName = "presets"

// ProfileDir is the directory new profiles are written to.
func ProfileDir() string {
	return filepath.Join(GlobalConfigDir(), ProfileDirName)
}

// ProfileDirs is every directory profiles are read from, in precedence order:
// the current one first, the legacy `presets/` second, so a profile that exists
// in both resolves to the current one.
func ProfileDirs() []string {
	return []string{ProfileDir(), filepath.Join(GlobalConfigDir(), LegacyProfileDirName)}
}

// ValidateProfileID rejects empty ids and ids with characters outside the
// [a-zA-Z0-9_-] set used for the on-disk profile file or directory name.
func ValidateProfileID(id string) error {
	if id == "" {
		return fmt.Errorf("profile ID cannot be empty")
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid profile ID: %s. Use only alphanumeric characters, dashes, and underscores", id)
		}
	}
	return nil
}

// ValidateCustomScript checks one script entry. File must be a bare file name —
// the generator turns it into a COPY inside the build context, so a directory
// part or a `..` would let a profile reach outside it.
func ValidateCustomScript(s types.CustomScript) error {
	if s.File == "" {
		return fmt.Errorf("custom script needs a file name")
	}
	if s.File != filepath.Base(s.File) || strings.ContainsAny(s.File, `/\`) || s.File == ".." {
		return fmt.Errorf("invalid custom script %q: must be a file name, not a path", s.File)
	}
	// The name is interpolated into the Dockerfile's COPY and into a single-quoted
	// shell word in the RUN that executes it, so the character set is kept to what
	// is safe in both instead of quoting after the fact.
	if !customScriptFileRe.MatchString(s.File) {
		return fmt.Errorf("invalid custom script %q: must be a .sh file named with letters, digits, dots, dashes or underscores", s.File)
	}
	if s.When != "" && !slices.Contains(types.ScriptWhens, s.When) {
		return fmt.Errorf("invalid 'when' %q for script %q: expected one of %s", s.When, s.File, scriptWhenList())
	}
	return nil
}

func scriptWhenList() string {
	out := make([]string, 0, len(types.ScriptWhens))
	for _, w := range types.ScriptWhens {
		out = append(out, string(w))
	}
	return strings.Join(out, ", ")
}

// ParseScriptSpec parses a `--script <path>[:when]` value into a CustomScript
// whose Source is the absolute host path to copy from. The `when` suffix is
// only recognised when it names a real ScriptWhen, so a path that happens to
// contain a colon still parses.
func ParseScriptSpec(spec string) (types.CustomScript, error) {
	path, when := spec, types.ScriptWhen("")
	if idx := strings.LastIndex(spec, ":"); idx > 0 {
		if candidate := types.ScriptWhen(spec[idx+1:]); slices.Contains(types.ScriptWhens, candidate) {
			path, when = spec[:idx], candidate
		}
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return types.CustomScript{}, fmt.Errorf("empty --script value")
	}
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return types.CustomScript{}, fmt.Errorf("invalid --script path %q: %w", path, err)
	}
	script := types.CustomScript{File: filepath.Base(abs), When: when, Source: abs}
	if err := ValidateCustomScript(script); err != nil {
		return types.CustomScript{}, err
	}
	return script, nil
}

// expandHome resolves a leading ~ against the user's home directory, which
// filepath.Abs would otherwise turn into a literal "./~" path.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
}

// ProfileScripts resolves a profile's script entries against the directory it
// was loaded from, validating each one and filling in Source so the caller can
// copy them into the build directory.
func ProfileScripts(p catalog.Profile) ([]types.CustomScript, error) {
	if len(p.Scripts) == 0 {
		return nil, nil
	}
	if p.Dir == "" {
		return nil, fmt.Errorf("profile %q declares scripts but has no directory to resolve them against", p.ID)
	}
	out := make([]types.CustomScript, 0, len(p.Scripts))
	for _, s := range p.Scripts {
		if err := ValidateCustomScript(s); err != nil {
			return nil, fmt.Errorf("profile %q: %w", p.ID, err)
		}
		s.Source = filepath.Join(p.Dir, s.File)
		out = append(out, s)
	}
	return out, nil
}

// CollectCustomScriptFiles returns every configured custom script's build-dir
// name, whatever its `when`. It is the list the build directory must contain
// and that feeds the image fingerprint.
func CollectCustomScriptFiles(config *types.DevcontainerConfig) ([]string, error) {
	build, start, manual, err := PartitionCustomScripts(config)
	if err != nil {
		return nil, err
	}
	return slices.Concat(build, start, manual), nil
}

// PartitionCustomScripts splits the configured scripts by when they run. The
// names returned are build-dir names (already prefixed), which is what both the
// Dockerfile and the build directory use.
func PartitionCustomScripts(config *types.DevcontainerConfig) (build, start, manual []string, err error) {
	seen := map[string]bool{}
	for _, s := range config.Dockerfile.Scripts {
		if err := ValidateCustomScript(s); err != nil {
			return nil, nil, nil, err
		}
		name := s.BuildFile()
		if seen[name] {
			continue
		}
		seen[name] = true
		switch s.ResolvedWhen() {
		case types.ScriptWhenStart:
			start = append(start, name)
		case types.ScriptWhenManual:
			manual = append(manual, name)
		default:
			build = append(build, name)
		}
	}
	return build, start, manual, nil
}
