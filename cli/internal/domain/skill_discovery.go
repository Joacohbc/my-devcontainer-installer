package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// DiscoveredSkill represents an agent skill detected from a repository or source.
type DiscoveredSkill struct {
	ID          types.SkillID
	Name        string
	Description string
	Source      string
	Snyk        string
	Status      string
	Manifest    types.UserSkillManifest
}

// SkippedSkill describes a skill from discovery that was omitted.
type SkippedSkill struct {
	Name   string
	Status string
	Reason string
}

type rawNpxSkillEntry struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Source   string `json:"source"`
	Path     string `json:"path"`
	Security struct {
		Gen     string `json:"gen"`
		Socket  string `json:"socket"`
		Snyk    string `json:"snyk"`
		Details string `json:"details"`
	} `json:"security"`
}

// NormaliseSkillID converts a human or repository skill name (e.g. "Convex Best Practices")
// into a normalized, valid skill identifier (e.g. "convex-best-practices") satisfying ValidateSkillID.
func NormaliseSkillID(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			lastDash = false
		} else if r == '-' || unicode.IsSpace(r) || unicode.IsPunct(r) {
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// ParseSkillFrontmatter extracts the name and description from the YAML frontmatter of SKILL.md.
func ParseSkillFrontmatter(content []byte) (name, description string, err error) {
	text := string(content)
	text = strings.TrimPrefix(text, "\ufeff")
	trimmed := strings.TrimLeft(text, " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return "", "", nil
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return "", "", nil
	}
	var yamlLines []string
	foundClosing := false
	for _, line := range lines[1:] {
		trimmedLine := strings.TrimRight(line, " \t\r")
		if trimmedLine == "---" {
			foundClosing = true
			break
		}
		yamlLines = append(yamlLines, line)
	}
	if !foundClosing {
		return "", "", nil
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(yamlLines, "\n")), &fm); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(fm.Name), strings.TrimSpace(fm.Description), nil
}

// ParseDiscoveredSkills parses the JSON array returned by the Skills CLI, reads each
// installed skill's SKILL.md frontmatter from its directory path, normalizes IDs,
// and builds the user skill manifests. Entries whose status != "installed" are reported in the skipped slice.
func ParseDiscoveredSkills(jsonData []byte, readSkillMD func(dirPath string) ([]byte, error), source string) ([]*DiscoveredSkill, []SkippedSkill, error) {
	var entries []rawNpxSkillEntry
	if err := json.Unmarshal(jsonData, &entries); err != nil {
		return nil, nil, fmt.Errorf("failed to parse skills discovery output: %w", err)
	}

	var discovered []*DiscoveredSkill
	var skipped []SkippedSkill

	for _, entry := range entries {
		if entry.Status != "installed" {
			skipped = append(skipped, SkippedSkill{
				Name:   entry.Name,
				Status: entry.Status,
				Reason: fmt.Sprintf("status %q is not installed", entry.Status),
			})
			continue
		}

		var desc string
		if readSkillMD != nil && entry.Path != "" {
			mdBytes, err := readSkillMD(entry.Path)
			if err == nil && len(mdBytes) > 0 {
				_, fmDesc, _ := ParseSkillFrontmatter(mdBytes)
				desc = fmDesc
			}
		}

		name := entry.Name
		id := NormaliseSkillID(name)
		if err := ValidateSkillID(id); err != nil {
			return nil, nil, fmt.Errorf("invalid skill ID %q for %q: %w", id, name, err)
		}

		manifest := types.UserSkillManifest{
			ID:    id,
			Label: name,
			Ref:   source,
			Skill: name,
		}
		if desc != "" {
			manifest.Context = &types.UserSkillContext{
				Title: name,
				Body:  desc,
			}
		}

		discovered = append(discovered, &DiscoveredSkill{
			ID:          types.SkillID(id),
			Name:        name,
			Description: desc,
			Source:      source,
			Snyk:        entry.Security.Snyk,
			Status:      entry.Status,
			Manifest:    manifest,
		})
	}

	return discovered, skipped, nil
}

// IsBuiltinSkill reports whether id names one of the skills shipped with the CLI.
func IsBuiltinSkill(id types.SkillID) bool {
	for _, s := range catalog.AgentSkills {
		if s.ID == id {
			return true
		}
	}
	return false
}
