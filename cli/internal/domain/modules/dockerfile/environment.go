package dockerfile

import (
	"fmt"
	"strings"
)

// EnvValue is a value in the container's environment. It may reference $HOME,
// which only a shell expands — every other consumer must read it through
// Expanded, so the two accessors below are the whole contract: a value never
// reaches a target without the caller stating which kind of target it is.
type EnvValue string

// ForShell returns the value as declared, for a target a shell will read.
func (v EnvValue) ForShell() string {
	return string(v)
}

// Expanded returns the value with $HOME resolved to devuser's home, for a
// target that performs no expansion of its own (a Dockerfile ENV instruction,
// which has no HOME at build time).
func (v EnvValue) Expanded() string {
	return strings.ReplaceAll(string(v), "$HOME", devuserHome)
}

// PathEntry is one directory prepended to the container's PATH. It is an
// EnvValue: same $HOME semantics, named for what it holds at the use site.
type PathEntry = EnvValue

// EnvVar is one variable a module sets in the image. It is the build-time
// counterpart of types.RequiredEnvVar, which is asked of the user at runtime.
type EnvVar struct {
	Name  string
	Value EnvValue
}

// ContainerEnv is the environment a module contributes to the image, declared
// as data rather than as shell text. Declaring it lets one value be rendered to
// every target that needs it — the Dockerfile's ENV, so a process started
// without any shell (`docker exec <binary>`) already has it, and a sourced
// script, so a shell that starts before inheriting that ENV still gets it. A
// module that wrote `export PATH=…` into a shell-init file instead would reach
// only the second.
//
// Assignments and PathEntries are separate because PATH accumulates across
// modules while every other variable is simply set.
//
// It carries no aliases or functions on purpose: a process cannot inherit
// those, so they belong to the shell-only channel (alias.sh) and would be
// silently useless here.
type ContainerEnv struct {
	Assignments []EnvVar
	PathEntries []PathEntry
}

// IsEmpty reports whether the module contributes nothing to the environment.
func (e ContainerEnv) IsEmpty() bool {
	return len(e.Assignments) == 0 && len(e.PathEntries) == 0
}

// EnvScriptFile is the single sourced script holding the whole environment,
// relative to devuser's home.
const EnvScriptFile = ".dc-env.sh"

// envScriptRcFiles are the startup files that source EnvScriptFile. They are
// not the rc files a module's shell-init uses: .zshenv is read by EVERY zsh,
// interactive or not, login or not, which is what makes `ssh <host> <command>`
// (a non-interactive `zsh -c`) see the environment. .profile covers login bash
// and sh, .bashrc non-login interactive bash.
var envScriptRcFiles = []string{".zshenv", ".profile", ".bashrc"}

// MergeContainerEnv folds every module's contribution into one environment,
// keeping first-declaration order. A PATH entry declared by two modules appears
// once; a variable assigned twice keeps its first position and its last value.
func MergeContainerEnv(envs ...ContainerEnv) ContainerEnv {
	merged := ContainerEnv{}
	positionOf := map[string]int{}
	declaredPath := map[PathEntry]bool{}
	for _, env := range envs {
		for _, assignment := range env.Assignments {
			position, alreadyDeclared := positionOf[assignment.Name]
			if alreadyDeclared {
				merged.Assignments[position] = assignment
				continue
			}
			positionOf[assignment.Name] = len(merged.Assignments)
			merged.Assignments = append(merged.Assignments, assignment)
		}
		for _, entry := range env.PathEntries {
			if declaredPath[entry] {
				continue
			}
			declaredPath[entry] = true
			merged.PathEntries = append(merged.PathEntries, entry)
		}
	}
	return merged
}

// RenderDockerfileEnv renders the environment as Dockerfile ENV instructions,
// the only form a process started without a shell inherits. Returns an empty
// string when there is nothing to declare.
func RenderDockerfileEnv(env ContainerEnv) string {
	if env.IsEmpty() {
		return ""
	}
	lines := []string{"##", "## ENVIRONMENT", "##"}
	for _, assignment := range env.Assignments {
		lines = append(lines, fmt.Sprintf("ENV %s=%s", assignment.Name, doubleQuoted(assignment.Value.Expanded())))
	}
	if len(env.PathEntries) > 0 {
		lines = append(lines, fmt.Sprintf("ENV PATH=%s", doubleQuoted(expandedPathValue(env.PathEntries))))
	}
	return strings.Join(lines, "\n")
}

// RenderEnvScript renders the environment as a POSIX script written to
// EnvScriptFile and sourced from envScriptRcFiles, for the shells that start
// before inheriting the image environment. Every PATH entry is prepended
// through a guard, so sourcing it twice — which a login bash does, since
// Ubuntu's .profile also sources .bashrc — cannot grow PATH.
func RenderEnvScript(env ContainerEnv) string {
	if env.IsEmpty() {
		return ""
	}
	var lines []string
	for _, assignment := range env.Assignments {
		lines = append(lines, fmt.Sprintf("export %s=%s", assignment.Name, doubleQuoted(assignment.Value.ForShell())))
	}
	for _, entry := range prependOrder(env.PathEntries) {
		lines = append(lines, pathPrependGuard(entry))
	}
	if len(env.PathEntries) > 0 {
		lines = append(lines, "export PATH")
	}
	return emitShellInitTo(envScriptRcFiles, EnvScriptFile, lines)
}

// prependOrder reverses the entries, so that prepending them one at a time
// leaves PATH in declaration order — the same order RenderDockerfileEnv writes.
func prependOrder(entries []PathEntry) []PathEntry {
	reversed := make([]PathEntry, len(entries))
	for i, entry := range entries {
		reversed[len(entries)-1-i] = entry
	}
	return reversed
}

func expandedPathValue(entries []PathEntry) string {
	expanded := make([]string, len(entries))
	for i, entry := range entries {
		expanded[i] = entry.Expanded()
	}
	return strings.Join(expanded, ":") + ":$PATH"
}

// pathPrependGuard yields a POSIX line that prepends entry to PATH only when it
// is not already there. It deliberately contains no single quote:
// emitShellInitTo wraps each line in them.
func pathPrependGuard(entry PathEntry) string {
	return fmt.Sprintf(`case ":$PATH:" in *":%s:"*) ;; *) PATH=%s:$PATH ;; esac`,
		entry.ForShell(), doubleQuoted(entry.ForShell()))
}

func doubleQuoted(value string) string {
	return `"` + value + `"`
}
