package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ProfileDirName is where user profiles are written and read from.
const ProfileDirName = "profiles"

// CustomScriptNamePattern is the accepted shape of a custom script's file name.
// It is exported so the catalogue dump an agent reads can state the rule rather
// than restate it in prose that could drift from the check below.
const CustomScriptNamePattern = `^[A-Za-z0-9][A-Za-z0-9._-]*\.sh$`

// customScriptFileRe is the accepted shape of a custom script's file name.
var customScriptFileRe = regexp.MustCompile(CustomScriptNamePattern)

// ProfileDir is the directory new profiles are written to.
func ProfileDir() string {
	return filepath.Join(GlobalConfigDir(), ProfileDirName)
}

// ProfileDirs is every directory profiles are read from. It is a list so the
// catalog API stays variadic, but only the current directory is read.
func ProfileDirs() []string {
	return []string{ProfileDir()}
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
	out := make([]types.CustomScript, 0, len(p.Scripts))
	for _, s := range p.Scripts {
		if err := ValidateCustomScript(s); err != nil {
			return nil, fmt.Errorf("profile %q: %w", p.ID, err)
		}
		if s.Source != "" {
			out = append(out, s)
			continue
		}
		if p.Dir == "" {
			return nil, fmt.Errorf("profile %q declares scripts but has no directory to resolve them against", p.ID)
		}
		s.Source = filepath.Join(p.Dir, s.File)
		out = append(out, s)
	}
	return out, nil
}

// ReadCustomScript returns a script's bytes from wherever it lives: the host
// filesystem for a user profile or --script.
func ReadCustomScript(s types.CustomScript) ([]byte, error) {
	if s.Source == "" {
		return nil, fmt.Errorf("custom script %q has no source to read from", s.File)
	}
	data, err := os.ReadFile(s.Source)
	if err != nil {
		return nil, fmt.Errorf("custom script %q: %w", s.File, err)
	}
	return data, nil
}

// projectPathRe matches the references that only resolve once a project is
// mounted: the mount root or the legacy /workspace mount (but not a workspaces
// dir under $HOME, hence the guard on the preceding character), and the
// contract variables or the ad-hoc $WORKSPACE scripts tend to invent.
var projectPathRe = regexp.MustCompile(`(?:^|[^\w}./~-])(/workspaces?)\b|(\$\{?(?:DC_WORKSPACE\w*|WORKSPACE)\b)`)

// LintCustomScript returns advisory warnings for a script whose content cannot
// work in the phase it declares. Today that is one mistake: a build script that
// touches the project, which does not exist inside `docker build`. Comment lines
// are skipped so a script may explain itself, and every offending line is folded
// into one warning so a long script does not flood each build.
func LintCustomScript(s types.CustomScript, content []byte) []string {
	if s.ResolvedWhen() != types.ScriptWhenBuild {
		return nil
	}
	var first string
	var lines []string
	for i, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		m := projectPathRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if first == "" {
			first = m[1] + m[2]
		}
		lines = append(lines, strconv.Itoa(i+1))
	}
	if len(lines) == 0 {
		return nil
	}
	where := "line " + lines[0]
	if len(lines) > 1 {
		where = "lines " + strings.Join(lines, ", ")
	}
	return []string{fmt.Sprintf(
		"script %s references %q (%s) but runs at build time, when no project is mounted; declare it 'when: start' and use $DC_WORKSPACE_DIR",
		s.File, first, where)}
}

// CollectCustomScriptFiles returns every configured custom script's build-dir
// name, whatever its `when`. It is the list the build directory must contain
// and that feeds the image fingerprint.
func CollectCustomScriptFiles(config *types.DevcontainerConfig) ([]string, error) {
	build, start, service, manual, err := PartitionCustomScripts(config)
	if err != nil {
		return nil, err
	}
	return slices.Concat(build, start, service, manual), nil
}

// PartitionCustomScripts splits the configured scripts by when they run. The
// names returned are build-dir names (already prefixed), which is what both the
// Dockerfile and the build directory use.
func PartitionCustomScripts(config *types.DevcontainerConfig) (build, start, service, manual []string, err error) {
	// Keyed by build-dir name so the check catches the one pair that is not an
	// exact duplicate: `x.sh` and `custom-x.sh` both resolve to `custom-x.sh`,
	// and letting that through would silently drop one script and build an image
	// missing it. Listing the same file twice stays harmless.
	seen := map[string]string{}
	for _, s := range config.Dockerfile.Scripts {
		if err := ValidateCustomScript(s); err != nil {
			return nil, nil, nil, nil, err
		}
		name := s.BuildFile()
		if prev, ok := seen[name]; ok {
			if prev != s.File {
				return nil, nil, nil, nil, fmt.Errorf("custom scripts %q and %q collide: both are staged as %q — rename one", prev, s.File, name)
			}
			continue
		}
		seen[name] = s.File
		switch s.ResolvedWhen() {
		case types.ScriptWhenStart:
			start = append(start, name)
		case types.ScriptWhenService:
			service = append(service, name)
		case types.ScriptWhenManual:
			manual = append(manual, name)
		default:
			build = append(build, name)
		}
	}
	return build, start, service, manual, nil
}

// ApplyProfileTemplate copies a profile's project-level settings into config:
// companion services (replacing the current set when the profile names any),
// extra volumes (added), environment variables (set) and, for a profile that
// starts from an image, mode=image with that image — or, when it also lists
// modules, a derived custom build FROM that image. Modules, scripts, ports and
// skills are applied by the caller, which also lets explicit flags win.
func ApplyProfileTemplate(config *types.DevcontainerConfig, p catalog.Profile) error {
	for _, svc := range p.Services {
		if catalog.GetComposeService(svc.ID) == nil {
			return fmt.Errorf("profile %q: %w", p.ID, unknownServiceError(string(svc.ID)))
		}
	}
	for k := range p.Env {
		if !envNamePattern.MatchString(k) {
			return fmt.Errorf("profile %q: invalid env variable name %q", p.ID, k)
		}
	}

	if len(p.Services) > 0 {
		config.Compose.Services = make([]types.SelectedService, 0, len(p.Services))
		for _, svc := range p.Services {
			if svc.Options == nil {
				svc.Options = map[string]any{}
			}
			config.Compose.Services = append(config.Compose.Services, svc)
		}
	}
	for _, v := range p.Volumes {
		if !slices.Contains(config.Compose.Volumes, v) {
			config.Compose.Volumes = append(config.Compose.Volumes, v)
		}
	}
	if len(p.Env) > 0 && config.Env == nil {
		config.Env = make(map[string]string, len(p.Env))
	}
	for k, v := range p.Env {
		config.Env[k] = v
	}
	if config.Mode == types.BuildModeProfiles {
		// A pulled profile image is fixed: the template never switches it to a
		// local build (an explicit --mode still can, after this).
		return nil
	}
	switch {
	case p.Image != "" && len(p.Modules) > 0:
		// Image plus modules is a derived build: the project image starts FROM
		// that image and adds only the modules it lacks.
		config.Mode = types.BuildModeCustom
		config.Dockerfile.From = p.Image
	case p.Image != "":
		config.Mode = types.BuildModeImage
		config.Image = ResolveImageRef(p.Image)
		config.Dockerfile.From = ""
	default:
		// A modules-only profile is a full build: it undoes a previous
		// profile's image or derivation instead of silently keeping it.
		config.Dockerfile.From = ""
		if config.Mode == types.BuildModeImage && len(p.Modules) > 0 {
			config.Mode = types.BuildModeCustom
		}
	}
	return nil
}

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ResolveImageRef turns a profile's image into the reference to run: the tag of
// a named image definition when one exists, or the value itself (a registry
// reference) otherwise.
func ResolveImageRef(image string) string {
	if def, ok, _ := ResolveImageDef(image); ok {
		return ImageTag(def)
	}
	return image
}
