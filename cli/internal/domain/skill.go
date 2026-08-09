package domain

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Host-side agent skill. The CLI ships two skills that must not be confused:
//
//   - skill-devcontainer-context.md is baked INTO the image and tells an agent
//     running inside a container where it is (see modules/dockerfile/aliases.go).
//   - skill-devcontainer-cli.md, handled here, is installed on the HOST and
//     tells an agent running on the user's machine how to drive this CLI:
//     generate a project, run commands in it, forward ports, tear it down.
//
// The host skill is a plain file drop — no Docker involved — so everything it
// needs is path resolution plus deciding whether an existing file is ours.

// HostSkillName is the skill id: both the frontmatter `name` of the document
// and the directory it is installed under.
const HostSkillName = "devcontainer-cli"

// HostSkillFile is the filename every agent expects a skill's body to have.
const HostSkillFile = "SKILL.md"

// HostSkillAsset is the embedded document installed as that skill.
const HostSkillAsset = "skill-devcontainer-cli.md"

// HostSkillMarker is carried by the shipped document and therefore by every
// copy the CLI installed. It is what tells a skill we wrote — possibly an older
// version of it — apart from a file the user put there under the same name,
// which is never overwritten without --force.
const HostSkillMarker = "devcontainer-cli:managed skill=devcontainer-cli"

// SkillAgent is one host-side agent that reads skills from a directory.
type SkillAgent struct {
	ID    string // selection id, e.g. "claude"
	Label string // human-readable name of the agent
	Dir   string // skills root, relative to the scope's base directory
}

// HostSkillAgents lists the agent skill directories the CLI installs into.
// They mirror the two directories the container entrypoint links the in-image
// skill into, so the same skill lands in the same places on both sides:
// ~/.claude/skills for Claude Code, and ~/.agents/skills, the cross-tool store
// Antigravity and the other AGENTS.md-style agents read.
var HostSkillAgents = []SkillAgent{
	{ID: "claude", Label: "Claude Code", Dir: filepath.Join(".claude", "skills")},
	{ID: "agents", Label: "Antigravity and other AGENTS.md agents", Dir: filepath.Join(".agents", "skills")},
}

// HostSkillAgentByID resolves an agent by its selection id.
func HostSkillAgentByID(id string) (SkillAgent, bool) {
	for _, a := range HostSkillAgents {
		if a.ID == id {
			return a, true
		}
	}
	return SkillAgent{}, false
}

// HostSkillAgentIDs returns every agent id in display order.
func HostSkillAgentIDs() []string {
	ids := make([]string, len(HostSkillAgents))
	for i, a := range HostSkillAgents {
		ids[i] = a.ID
	}
	return ids
}

// ResolveHostSkillAgents maps selection ids to agents, keeping the catalog's
// order and rejecting unknown ids. An empty selection means every agent.
func ResolveHostSkillAgents(ids []string) ([]SkillAgent, error) {
	if len(ids) == 0 {
		return HostSkillAgents, nil
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		if _, ok := HostSkillAgentByID(id); !ok {
			return nil, fmt.Errorf("unknown agent %q. Expected one of: %v", id, HostSkillAgentIDs())
		}
		wanted[id] = true
	}
	var out []SkillAgent
	for _, a := range HostSkillAgents {
		if wanted[a.ID] {
			out = append(out, a)
		}
	}
	return out, nil
}

// Skill install scopes: the user's home (every project) or a single project
// directory (only agents started there).
const (
	SkillScopeGlobal  = "global"
	SkillScopeProject = "project"
)

// SkillScopes lists the accepted --scope values in display order.
var SkillScopes = []string{SkillScopeGlobal, SkillScopeProject}

// HostSkillBaseDir returns the directory the agents' skill dirs hang off for a
// scope: the home directory for "global", cwd for "project".
func HostSkillBaseDir(scope, cwd string) (string, error) {
	switch scope {
	case SkillScopeGlobal, "":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		return home, nil
	case SkillScopeProject:
		return cwd, nil
	default:
		return "", fmt.Errorf("unknown scope %q. Expected one of: %v", scope, SkillScopes)
	}
}

// HostSkillDir returns the directory the skill is installed in for an agent.
func HostSkillDir(base string, agent SkillAgent) string {
	return filepath.Join(base, agent.Dir, HostSkillName)
}

// HostSkillPath returns the full path of the installed SKILL.md for an agent.
func HostSkillPath(base string, agent SkillAgent) string {
	return filepath.Join(HostSkillDir(base, agent), HostSkillFile)
}

// SkillState is the result of comparing an install target with the document the
// binary ships.
type SkillState string

const (
	// SkillAbsent: nothing installed at the target path.
	SkillAbsent SkillState = "absent"
	// SkillCurrent: our document, byte-identical to the shipped one.
	SkillCurrent SkillState = "current"
	// SkillOutdated: our document, but from an older CLI version.
	SkillOutdated SkillState = "outdated"
	// SkillForeign: a file exists that the CLI did not write; installing over
	// it needs --force.
	SkillForeign SkillState = "foreign"
)

// ClassifyHostSkill reports what is installed at path relative to want, the
// document this binary ships.
func ClassifyHostSkill(path string, want []byte) SkillState {
	got, err := os.ReadFile(path)
	if err != nil {
		return SkillAbsent
	}
	if bytes.Equal(got, want) {
		return SkillCurrent
	}
	if bytes.Contains(got, []byte(HostSkillMarker)) {
		return SkillOutdated
	}
	return SkillForeign
}
