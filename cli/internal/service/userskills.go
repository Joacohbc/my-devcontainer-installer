package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// LocateUserSkills validates that every id is a user-defined skill and returns
// their manifest paths, so a command can fail before it prompts.
func (s ConfigService) LocateUserSkills(ids []string) ([]string, error) {
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		path, ok := domain.UserSkillPath(id)
		if !ok {
			if domain.IsBuiltinSkill(types.SkillID(id)) {
				return nil, fmt.Errorf("skill %q is a built-in skill and cannot be removed", id)
			}
			return nil, fmt.Errorf("user-defined skill %q not found", id)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// RemoveUserSkills deletes the given user-defined skills and records one commit
// for the lot in the configuration repository. Every id is validated first.
func (s ConfigService) RemoveUserSkills(ids []string) error {
	paths, err := s.LocateUserSkills(ids)
	if err != nil {
		return err
	}
	repo := NewConfigRepo(reporterOrSilent(s.Report))
	versioned := repo.Prepare()
	for i, path := range paths {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove skill %q: %w", ids[i], err)
		}
	}
	if versioned {
		repo.Commit("chore(skill): remove "+strings.Join(ids, ", "), paths...)
	}
	return nil
}
