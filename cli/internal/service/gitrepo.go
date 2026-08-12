package service

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/git"
)

// GitRepoService initialises a project's git repository when the user has asked
// the CLI to offer that.
type GitRepoService struct {
	Report Reporter
	Prompt Prompter
}

// EnsureRepo creates a repository in dir when the global `git-init` setting is
// on and dir is not one yet, and reports whether it created one.
//
// It writes into the user's own directory rather than into the CLI's own
// `.dc_<ws>/`, which is why nothing happens unless the setting was turned on,
// and why an interactive run still asks. With the setting on and no terminal to
// ask at, the opt-in already answered the question.
func (s GitRepoService) EnsureRepo(dir string, interactive bool) (bool, error) {
	if !domain.GitInitEnabled() || git.IsRepo(dir) {
		return false, nil
	}
	if !git.IsAvailable() {
		s.Report.Warn("git-init is on but git is not installed; leaving %s as it is.", dir)
		return false, nil
	}

	if interactive {
		proceed, err := s.Prompt.Confirm("This directory is not a git repository. Initialise one?")
		if err != nil {
			return false, err
		}
		if !proceed {
			return false, nil
		}
	}

	if err := git.Init(dir); err != nil {
		return false, err
	}
	s.Report.Success("Initialised a git repository in %s", dir)
	return true, nil
}
