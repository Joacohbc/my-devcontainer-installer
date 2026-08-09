package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
)

// SkillService installs the host-side agent skill — the document that teaches
// an assistant running on the user's machine (Claude Code, Antigravity, …) how
// to drive this CLI. It is pure file I/O: no Docker, no container involved.
type SkillService struct {
	Report Reporter
}

// SkillTargetStatus is one agent's install target and what is currently there.
type SkillTargetStatus struct {
	Agent domain.SkillAgent
	Path  string
	State domain.SkillState
}

// Installed reports whether the target holds a skill the CLI wrote (current or
// from an older version).
func (s SkillTargetStatus) Installed() bool {
	return s.State == domain.SkillCurrent || s.State == domain.SkillOutdated
}

// SkillDocument returns the skill document this binary ships.
func (s SkillService) SkillDocument() ([]byte, error) {
	content, err := assets.ReadAsset(domain.HostSkillAsset)
	if err != nil {
		return nil, fmt.Errorf("reading embedded skill %s: %w", domain.HostSkillAsset, err)
	}
	return content, nil
}

// Status reports the install state of every requested agent under base. An
// empty agents slice means every known agent.
func (s SkillService) Status(base string, agents []domain.SkillAgent) ([]SkillTargetStatus, error) {
	content, err := s.SkillDocument()
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		agents = domain.HostSkillAgents
	}
	out := make([]SkillTargetStatus, 0, len(agents))
	for _, agent := range agents {
		path := domain.HostSkillPath(base, agent)
		out = append(out, SkillTargetStatus{
			Agent: agent,
			Path:  path,
			State: domain.ClassifyHostSkill(path, content),
		})
	}
	return out, nil
}

// Install writes the skill into every requested agent directory under base.
// A target holding a file the CLI did not write is left alone unless force is
// set, so a user's own skill of the same name is never silently replaced.
func (s SkillService) Install(base string, agents []domain.SkillAgent, force bool) error {
	content, err := s.SkillDocument()
	if err != nil {
		return err
	}
	targets, err := s.Status(base, agents)
	if err != nil {
		return err
	}
	installed := 0
	for _, target := range targets {
		if target.State == domain.SkillForeign && !force {
			s.Report.Warn("Skipped %s: %s already exists and was not written by this CLI (pass --force to replace it)", target.Agent.Label, target.Path)
			continue
		}
		if err := writeSkillFile(target.Path, content); err != nil {
			return err
		}
		installed++
		switch target.State {
		case domain.SkillCurrent:
			s.Report.Info("%s: already up to date (%s)", target.Agent.Label, target.Path)
		case domain.SkillOutdated:
			s.Report.Success("%s: updated %s", target.Agent.Label, target.Path)
		default:
			s.Report.Success("%s: installed %s", target.Agent.Label, target.Path)
		}
	}
	if installed == 0 {
		return fmt.Errorf("no skill was installed")
	}
	s.Report.Info("Agents pick the skill up on their next session; ask yours to use the '%s' skill.", domain.HostSkillName)
	return nil
}

// Remove deletes the installed skill from every requested agent directory,
// leaving a file the CLI did not write in place unless force is set. Removing
// something that is not installed is not an error.
func (s SkillService) Remove(base string, agents []domain.SkillAgent, force bool) error {
	targets, err := s.Status(base, agents)
	if err != nil {
		return err
	}
	for _, target := range targets {
		switch {
		case target.State == domain.SkillAbsent:
			s.Report.Info("%s: nothing installed at %s", target.Agent.Label, target.Path)
		case target.State == domain.SkillForeign && !force:
			s.Report.Warn("Skipped %s: %s was not written by this CLI (pass --force to remove it anyway)", target.Agent.Label, target.Path)
		default:
			if err := os.Remove(target.Path); err != nil {
				return fmt.Errorf("removing %s: %w", target.Path, err)
			}
			// The skill's own directory is ours; drop it when it is empty so no
			// dangling devcontainer-cli/ folder is left in the agent's store.
			_ = os.Remove(filepath.Dir(target.Path))
			s.Report.Success("%s: removed %s", target.Agent.Label, target.Path)
		}
	}
	return nil
}

func writeSkillFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
