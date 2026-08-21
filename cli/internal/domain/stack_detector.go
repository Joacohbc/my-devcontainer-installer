package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

type DetectedStack struct {
	Modules       []types.ModuleID
	Services      []types.ServiceID
	HasSkillsLock bool
	DetectedFiles []string
}

func IsGitRepoURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "git@") ||
		strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "ssh://") ||
		strings.HasPrefix(trimmed, "git://") {
		return true
	}
	if strings.HasSuffix(trimmed, ".git") {
		return true
	}
	if strings.Contains(trimmed, "github.com/") ||
		strings.Contains(trimmed, "gitlab.com/") ||
		strings.Contains(trimmed, "bitbucket.org/") {
		return true
	}
	return false
}

func ExtractRepoName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	if idx := strings.LastIndex(trimmed, ":"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func DetectStack(dir string) (DetectedStack, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return DetectedStack{}, err
	}

	presentFiles := make(map[string]bool, len(entries))
	for _, entry := range entries {
		presentFiles[entry.Name()] = true
	}

	stack := DetectedStack{}

	detectNodeStack(dir, presentFiles, &stack)
	detectGoStack(presentFiles, &stack)
	detectPythonStack(presentFiles, &stack)
	detectJavaStack(presentFiles, &stack)
	detectPhpStack(presentFiles, &stack)
	detectRustStack(presentFiles, &stack)
	detectCCppStack(presentFiles, &stack)
	detectSqliteStack(presentFiles, &stack)
	detectGithubStack(presentFiles, &stack)
	detectDodStack(presentFiles, &stack)
	detectSkillsStack(dir, presentFiles, &stack)

	stack.Modules = deduplicateModules(stack.Modules)
	stack.Services = deduplicateServices(stack.Services)
	slices.Sort(stack.DetectedFiles)

	return stack, nil
}

func detectNodeStack(dir string, present map[string]bool, stack *DetectedStack) {
	isNodeProject := present["package.json"] || present["tsconfig.json"] || present["jsconfig.json"]
	if !isNodeProject {
		return
	}

	stack.Modules = append(stack.Modules, types.ModuleID("nodejs"))
	if present["package.json"] {
		stack.DetectedFiles = append(stack.DetectedFiles, "package.json")
		detectPackageManagerFromPackageJSON(dir, stack)
	}
	if present["tsconfig.json"] {
		stack.DetectedFiles = append(stack.DetectedFiles, "tsconfig.json")
	}
	if present["jsconfig.json"] {
		stack.DetectedFiles = append(stack.DetectedFiles, "jsconfig.json")
	}

	if present["pnpm-lock.yaml"] || present["pnpm-workspace.yaml"] {
		stack.Modules = append(stack.Modules, types.ModuleID("pnpm"))
		if present["pnpm-lock.yaml"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "pnpm-lock.yaml")
		}
		if present["pnpm-workspace.yaml"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "pnpm-workspace.yaml")
		}
	}

	if present["yarn.lock"] || present[".yarnrc"] || present[".yarnrc.yml"] {
		stack.Modules = append(stack.Modules, types.ModuleID("yarn"))
		if present["yarn.lock"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "yarn.lock")
		}
	}

	if present["bun.lockb"] || present["bun.lock"] || present["bunfig.toml"] {
		stack.Modules = append(stack.Modules, types.ModuleID("bun"))
		if present["bun.lockb"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "bun.lockb")
		}
		if present["bun.lock"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "bun.lock")
		}
	}
}

type packageJSONHeader struct {
	PackageManager string `json:"packageManager"`
}

func detectPackageManagerFromPackageJSON(dir string, stack *DetectedStack) {
	content, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}
	var header packageJSONHeader
	if err := json.Unmarshal(content, &header); err != nil {
		return
	}
	manager := strings.ToLower(header.PackageManager)
	if strings.HasPrefix(manager, "pnpm") {
		stack.Modules = append(stack.Modules, types.ModuleID("pnpm"))
	} else if strings.HasPrefix(manager, "yarn") {
		stack.Modules = append(stack.Modules, types.ModuleYarn)
	} else if strings.HasPrefix(manager, "bun") {
		stack.Modules = append(stack.Modules, types.ModuleBun)
	}
}

func detectGoStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "go.mod", "go.sum", "go.work", ".go")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleGolang)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectPythonStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "requirements.txt", "pyproject.toml", "Pipfile", "Pipfile.lock", "setup.py", "setup.cfg", "uv.lock", "poetry.lock", "environment.yml", ".py")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModulePython)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectJavaStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "pom.xml", "mvnw", "mvnw.cmd", "build.gradle", "build.gradle.kts", "gradlew", "gradlew.bat", "settings.gradle", "settings.gradle.kts", ".java")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleJavaTemurin)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectPhpStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "composer.json", "composer.lock", ".php")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModulePhp)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectRustStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "Cargo.toml", "Cargo.lock", ".rs")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleRust)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectCCppStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "CMakeLists.txt", "Makefile", "meson.build", ".c", ".cpp", ".cc", ".h", ".hpp")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleCCpp)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectSqliteStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyExtension(present, ".sqlite", ".sqlite3", ".db")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleSqlite)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectGithubStack(present map[string]bool, stack *DetectedStack) {
	if !present[".github"] {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleGithubCli)
	stack.DetectedFiles = append(stack.DetectedFiles, ".github")
}

func detectDodStack(present map[string]bool, stack *DetectedStack) {
	matched := matchAnyFile(present, "Dockerfile", "docker-compose.yml", "compose.yaml", "compose.yml")
	if len(matched) == 0 {
		return
	}
	stack.Modules = append(stack.Modules, types.ModuleDod)
	stack.DetectedFiles = append(stack.DetectedFiles, matched...)
}

func detectSkillsStack(dir string, present map[string]bool, stack *DetectedStack) {
	hasRootSkills := present["skills-lock.json"] || present[".skills-lock.json"] || present["skills.json"]
	if hasRootSkills {
		stack.HasSkillsLock = true
		stack.Modules = append(stack.Modules, types.ModuleNodejs)
		if present["skills-lock.json"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "skills-lock.json")
		}
		if present[".skills-lock.json"] {
			stack.DetectedFiles = append(stack.DetectedFiles, ".skills-lock.json")
		}
		if present["skills.json"] {
			stack.DetectedFiles = append(stack.DetectedFiles, "skills.json")
		}
		return
	}

	nestedCandidates := []string{
		filepath.Join(".agents", "skills-lock.json"),
		filepath.Join(".claude", "skills-lock.json"),
	}
	for _, candidate := range nestedCandidates {
		if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
			stack.HasSkillsLock = true
			stack.Modules = append(stack.Modules, types.ModuleID("nodejs"))
			stack.DetectedFiles = append(stack.DetectedFiles, candidate)
			return
		}
	}
}

func matchAnyFile(present map[string]bool, names ...string) []string {
	var matched []string
	for _, name := range names {
		if strings.HasPrefix(name, ".") {
			extMatches := matchAnyExtension(present, name)
			matched = append(matched, extMatches...)
			continue
		}
		if present[name] {
			matched = append(matched, name)
		}
	}
	return matched
}

func matchAnyExtension(present map[string]bool, exts ...string) []string {
	var matched []string
	for fileName := range present {
		for _, ext := range exts {
			if strings.HasSuffix(fileName, ext) {
				matched = append(matched, fileName)
				break
			}
		}
	}
	return matched
}

func deduplicateModules(modules []types.ModuleID) []types.ModuleID {
	seen := make(map[types.ModuleID]bool, len(modules))
	var unique []types.ModuleID
	for _, m := range modules {
		if seen[m] {
			continue
		}
		seen[m] = true
		unique = append(unique, m)
	}
	return unique
}

func deduplicateServices(services []types.ServiceID) []types.ServiceID {
	seen := make(map[types.ServiceID]bool, len(services))
	var unique []types.ServiceID
	for _, s := range services {
		if seen[s] {
			continue
		}
		seen[s] = true
		unique = append(unique, s)
	}
	return unique
}

func NormalizeModuleID(raw string) types.ModuleID {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case "golang":
		return types.ModuleGolang
	case "java", "temurin", "java-temurin":
		return types.ModuleJavaTemurin
	case "openjdk", "java-openjdk":
		return types.ModuleJavaOpenjdk
	case "cpp", "c", "c++", "c-cpp":
		return types.ModuleCCpp
	case "gh", "github", "github-cli":
		return types.ModuleGithubCli
	default:
		return types.ModuleID(normalized)
	}
}

func NormalizeServiceID(raw string) types.ServiceID {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return types.ServiceID(normalized)
}
