package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/skillsdiscovery"
)

// SkillDiscoveryRunner defines the contract for executing Skills CLI discovery.
type SkillDiscoveryRunner interface {
	DiscoverSkills(source string) (*skillsdiscovery.DiscoveredRaw, error)
}

func (s ConfigService) discoveryRunner() SkillDiscoveryRunner {
	if s.DiscoveryRunner != nil {
		return s.DiscoveryRunner
	}
	return skillsdiscovery.NewRunner()
}

// DiscoverSkills queries source using the Skills CLI, parses discovered skills,
// logs warnings for skipped items, and returns the discovered skills list.
func (s ConfigService) DiscoverSkills(source string) ([]*domain.DiscoveredSkill, error) {
	if strings.HasPrefix(source, "-") {
		return nil, fmt.Errorf("invalid source %q: source cannot start with '-'", source)
	}

	payload, err := s.discoveryRunner().DiscoverSkills(source)
	if err != nil {
		return nil, err
	}

	readMD := func(path string) ([]byte, error) {
		if payload != nil && payload.SkillMDs != nil {
			if data, ok := payload.SkillMDs[path]; ok {
				return data, nil
			}
		}
		return nil, fmt.Errorf("skill doc not found at path %q", path)
	}

	discovered, skipped, err := domain.ParseDiscoveredSkills(payload.RawJSON, readMD, source)
	if err != nil {
		return nil, err
	}

	for _, sk := range skipped {
		if s.Report != nil {
			s.Report.Warn("Skipping %s (%s)", sk.Name, sk.Reason)
		}
	}

	if len(discovered) == 0 {
		return nil, fmt.Errorf("no skills found in %s", source)
	}
	return discovered, nil
}

// SaveUserSkill validates and writes a single user-defined skill manifest to
// ~/.devcontainer-cli/skills/<id>.yml. Refuses to overwrite an existing user skill unless force is true.
func (s ConfigService) SaveUserSkill(manifest types.UserSkillManifest, force bool) error {
	id := manifest.ID
	if err := domain.ValidateSkillID(id); err != nil {
		return err
	}

	targetPath := filepath.Join(domain.SkillDir(), id+".yml")
	if _, exists := domain.UserSkillPath(id); exists && !force {
		return fmt.Errorf("user-defined skill %q already exists; pass --force to overwrite", id)
	}

	if domain.IsBuiltinSkill(types.SkillID(id)) && s.Report != nil {
		s.Report.Info("Note: %q is also a built-in skill id; yours takes precedence.", id)
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal skill manifest: %w", err)
	}

	if err := os.MkdirAll(domain.SkillDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to save skill: %w", err)
	}

	if s.Report != nil {
		s.Report.Success("Skill %q successfully created at %s", id, targetPath)
	}
	return nil
}

// SaveUserSkills writes a slice of discovered skills to ~/.devcontainer-cli/skills/<id>.yml.
// If a user skill with the same id exists and force is false, it skips that skill with a warning,
// delegating actual persistence to SaveUserSkill.
func (s ConfigService) SaveUserSkills(selected []*domain.DiscoveredSkill, force bool) error {
	for _, skill := range selected {
		id := string(skill.ID)
		if _, exists := domain.UserSkillPath(id); exists && !force {
			if s.Report != nil {
				s.Report.Warn("Skill %q already exists; skipping (use --force to overwrite)", id)
			}
			continue
		}
		if err := s.SaveUserSkill(skill.Manifest, force); err != nil {
			return err
		}
	}
	return nil
}
