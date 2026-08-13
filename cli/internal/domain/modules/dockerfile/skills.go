package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

const skillsInstallerAsset = "install-project-skills.sh"

// SkillsModule is the plumbing behind the project's agent skills: the installer
// on PATH, its alias, and the entrypoint hook that runs it on start.
//
// It has no UICategory because it is never picked directly — the generator adds
// it whenever the project selects skills. Requiring nodejs is what guarantees
// the npx the installer runs on is there, rather than discovering it is missing
// inside a container.
var SkillsModule = &ModuleSpec{
	ID:       types.ModuleSkills,
	Label:    "Agent skills (project-scoped, via the Skills CLI)",
	Category: types.CategoryInfra,
	Internal: true,
	Requires: []types.ModuleID{types.ModuleNodejs},
	CopyFiles: []string{
		skillsInstallerAsset,
	},
	PostScriptFiles: func(opts map[string]any) []string {
		return []string{"autostart-project-skills.sh"}
	},
	// The skills go into the project, so they are only worth installing once the
	// tools they describe are there.
	PostScriptAutoStart:  true,
	PostScriptStartOrder: 95,
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Agent skills",
			Body: ctxBody(
				"This project's agent skills are installed **into the workspace**, not into",
				"your home: they belong to the project and are visible to an agent started",
				"in it.",
				"",
				fmt.Sprintf("Install or refresh them with `%s` (aliased `%s`). Which skills and",
					types.SkillsInstallCommand, types.SkillsInstallAlias),
				fmt.Sprintf("whether they install automatically on start come from `%s` and `%s`,",
					types.SkillsEnvVar, types.SkillsModeEnvVar),
				"so changing either is a compose change rather than an image rebuild.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		installerPath := types.DevUserHome + "/.local/bin/" + types.SkillsInstallCommand
		aliasLine := fmt.Sprintf("alias %s=%s", types.SkillsInstallAlias, types.SkillsInstallCommand)
		return fmt.Sprintf(`##
## AGENT SKILLS (project-scoped)
##
COPY %s %s
RUN chmod +x %s && chown devuser:devuser %s

%s
`, skillsInstallerAsset, installerPath, installerPath, installerPath,
			emitShellInit(".skills_init.sh", []string{aliasLine}))
	},
}
