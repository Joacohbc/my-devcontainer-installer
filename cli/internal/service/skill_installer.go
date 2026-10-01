package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/skills"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// DefaultSkillInstallerImage is the lightweight Node image used by default.
const DefaultSkillInstallerImage = "node:22-alpine"

// SkillInstallOptions configures the ephemeral skill installer run.
type SkillInstallOptions struct {
	Dir         string
	Image       string
	All         bool
	Interactive bool
}

// SkillInstallService installs agent skills into a workspace using an ephemeral container.
type SkillInstallService struct {
	Report Reporter
	Prompt Prompter
}

// Install installs agent skills into the project workspace directory.
func (s SkillInstallService) Install(skillIDs []string, opts SkillInstallOptions) error {
	workspaceDir, resolveErr := resolveWorkspaceDirectory(opts.Dir)
	if resolveErr != nil {
		return resolveErr
	}

	requestedSpecs, resolveSkillsErr := s.resolveRequestedSkills(skillIDs, workspaceDir, opts.All, opts.Interactive)
	if resolveSkillsErr != nil {
		return resolveSkillsErr
	}
	if len(requestedSpecs) == 0 {
		s.Report.Info("No skills selected for installation.")
		return nil
	}

	installCommands := buildInstallCommands(requestedSpecs)
	if len(installCommands) == 0 {
		return nil
	}

	imageName := resolveInstallerImage(opts.Image)
	return s.runInstallerContainer(workspaceDir, imageName, installCommands, opts.Interactive)
}

func resolveWorkspaceDirectory(dir string) (string, error) {
	if dir == "" {
		return os.Getwd()
	}
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		return "", fmt.Errorf("target directory %q does not exist: %w", absPath, statErr)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("target path %q is not a directory", absPath)
	}
	return absPath, nil
}

func resolveInstallerImage(image string) string {
	if image != "" {
		return image
	}
	return DefaultSkillInstallerImage
}

func (s SkillInstallService) resolveRequestedSkills(skillIDs []string, workspaceDir string, shouldInstallAll, isInteractive bool) ([]*skills.Spec, error) {
	explicitIDs := parseSkillIDTokens(skillIDs)
	if len(explicitIDs) > 0 && shouldInstallAll {
		return nil, fmt.Errorf("cannot specify both skill IDs and --all")
	}
	if shouldInstallAll {
		return catalog.AllAgentSkills(domain.SkillDirs()...), nil
	}
	if len(explicitIDs) > 0 {
		return resolveExplicitSkills(explicitIDs)
	}

	configuredSpecs, hasConfiguredSkills, configErr := loadConfiguredProjectSkills(workspaceDir)
	if configErr != nil {
		return nil, configErr
	}
	if hasConfiguredSkills {
		return configuredSpecs, nil
	}
	if !isInteractive || s.Prompt == nil {
		return nil, fmt.Errorf("no skills specified and no skills configured in %s; specify skill IDs or pass --all", types.ConfigFile)
	}
	return s.promptSkillSelection()
}

func parseSkillIDTokens(skillIDs []string) []types.SkillID {
	var expandedIDs []types.SkillID
	for _, rawID := range skillIDs {
		expandedIDs = appendTokens(expandedIDs, rawID)
	}
	return expandedIDs
}

func appendTokens(existing []types.SkillID, rawID string) []types.SkillID {
	tokens := strings.Split(rawID, ",")
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed != "" {
			existing = append(existing, types.SkillID(trimmed))
		}
	}
	return existing
}

func resolveExplicitSkills(expandedIDs []types.SkillID) ([]*skills.Spec, error) {
	skillDirs := domain.SkillDirs()
	expandedList := catalog.ExpandSkillGroups(expandedIDs)
	var resolvedSpecs []*skills.Spec
	seenIDs := make(map[types.SkillID]bool, len(expandedList))

	for _, skillID := range expandedList {
		spec := catalog.GetAgentSkill(skillID, skillDirs...)
		if spec == nil {
			return nil, fmt.Errorf("unknown skill: %s", skillID)
		}
		if !seenIDs[spec.ID] {
			seenIDs[spec.ID] = true
			resolvedSpecs = append(resolvedSpecs, spec)
		}
	}
	return resolvedSpecs, nil
}

func loadConfiguredProjectSkills(workspaceDir string) ([]*skills.Spec, bool, error) {
	projectConfig, err := domain.LoadConfig(workspaceDir)
	if err != nil {
		return nil, false, err
	}
	if projectConfig == nil || len(projectConfig.Skills.Skills) == 0 {
		return nil, false, nil
	}

	cleanedIDs := filterNonEmptySkillIDs(projectConfig.Skills.Skills)
	if len(cleanedIDs) == 0 {
		return nil, false, nil
	}

	skillDirs := domain.SkillDirs()
	expandedIDs := catalog.ExpandSkillGroups(cleanedIDs)
	var resolvedSpecs []*skills.Spec
	seenIDs := make(map[types.SkillID]bool, len(expandedIDs))

	for _, skillID := range expandedIDs {
		spec := catalog.GetAgentSkill(skillID, skillDirs...)
		if spec == nil {
			return nil, false, fmt.Errorf("unknown skill in %s: %s", types.ConfigFile, skillID)
		}
		if !seenIDs[spec.ID] {
			seenIDs[spec.ID] = true
			resolvedSpecs = append(resolvedSpecs, spec)
		}
	}
	return resolvedSpecs, len(resolvedSpecs) > 0, nil
}

func filterNonEmptySkillIDs(rawIDs []types.SkillID) []types.SkillID {
	var filtered []types.SkillID
	for _, id := range rawIDs {
		trimmed := strings.TrimSpace(string(id))
		if trimmed != "" {
			filtered = append(filtered, types.SkillID(trimmed))
		}
	}
	return filtered
}

func (s SkillInstallService) promptSkillSelection() ([]*skills.Spec, error) {
	skillDirs := domain.SkillDirs()
	allAvailableSkills := catalog.AllAgentSkills(skillDirs...)
	options := buildSkillPromptOptions(allAvailableSkills)

	selectedOptions, promptErr := s.Prompt.Multiselect("Select agent skills to install:", options, nil)
	if promptErr != nil {
		return nil, promptErr
	}
	if len(selectedOptions) == 0 {
		return nil, nil
	}

	return resolvePromptSelectedSkills(selectedOptions, skillDirs), nil
}

func buildSkillPromptOptions(availableSkills []*skills.Spec) []Option {
	options := make([]Option, 0, len(availableSkills))
	for _, skill := range availableSkills {
		optionLabel := string(skill.ID)
		if skill.Label != "" {
			optionLabel = fmt.Sprintf("%s (%s)", skill.ID, skill.Label)
		}
		options = append(options, Option{
			Value: string(skill.ID),
			Label: optionLabel,
		})
	}
	return options
}

func resolvePromptSelectedSkills(selectedOptions []Option, skillDirs []string) []*skills.Spec {
	var resolvedSpecs []*skills.Spec
	for _, option := range selectedOptions {
		spec := catalog.GetAgentSkill(types.SkillID(option.Value), skillDirs...)
		if spec != nil {
			resolvedSpecs = append(resolvedSpecs, spec)
		}
	}
	return resolvedSpecs
}

type skillSourceGroup struct {
	source string
	isBare bool
	names  []string
}

func buildInstallCommands(specs []*skills.Spec) []string {
	var groups []*skillSourceGroup
	groupBySource := make(map[string]*skillSourceGroup)

	for _, spec := range specs {
		group := ensureSkillSourceGroup(&groups, groupBySource, spec.Ref)
		if spec.Skill == "" {
			group.isBare = true
			continue
		}
		if !slices.Contains(group.names, spec.Skill) {
			group.names = append(group.names, spec.Skill)
		}
	}

	commands := make([]string, 0, len(groups))
	for _, group := range groups {
		commands = append(commands, formatGroupInstallCommand(group))
	}
	return commands
}

func ensureSkillSourceGroup(groups *[]*skillSourceGroup, groupBySource map[string]*skillSourceGroup, sourceRef string) *skillSourceGroup {
	group, exists := groupBySource[sourceRef]
	if !exists {
		group = &skillSourceGroup{source: sourceRef}
		groupBySource[sourceRef] = group
		*groups = append(*groups, group)
	}
	return group
}

func formatGroupInstallCommand(group *skillSourceGroup) string {
	if group.isBare || len(group.names) == 0 {
		return fmt.Sprintf("npx --yes skills add %s", group.source)
	}
	skillArguments := make([]string, 0, len(group.names))
	for _, name := range group.names {
		skillArguments = append(skillArguments, fmt.Sprintf("--skill %s", name))
	}
	return fmt.Sprintf("npx --yes skills add %s %s", group.source, strings.Join(skillArguments, " "))
}

func (s SkillInstallService) runInstallerContainer(workspaceDir, imageName string, installCommands []string, isInteractive bool) error {
	if err := docker.EnsureDocker(); err != nil {
		return err
	}

	commandScript := strings.Join(installCommands, " && ")
	containerArgs := buildDockerRunArgs(workspaceDir, imageName, commandScript, isInteractive)

	s.Report.Info("Installing agent skills in %s using %s...", workspaceDir, imageName)

	if isInteractive {
		return s.runInteractiveContainer(containerArgs)
	}
	return s.runNonInteractiveContainer(containerArgs)
}

func buildDockerRunArgs(workspaceDir, imageName, commandScript string, isInteractive bool) []string {
	args := []string{
		"run", "--rm",
	}
	if isInteractive {
		args = append(args, "-i")
	}
	args = append(args,
		"-v", fmt.Sprintf("%s:/workspace", workspaceDir),
		"-w", "/workspace",
		"-e", "HOME=/tmp",
	)

	hostOwner := hostOwnerString()
	if hostOwner != "" {
		args = append(args, "--user", hostOwner)
	}

	args = append(args, imageName, "sh", "-c", commandScript)
	return args
}

func (s SkillInstallService) runInteractiveContainer(containerArgs []string) error {
	exitStatus, runErr := docker.DockerInherit(containerArgs)
	if runErr != nil {
		return runErr
	}
	if exitStatus != 0 {
		return fmt.Errorf("skill installer failed with exit status %d", exitStatus)
	}
	s.Report.Success("✓ Agent skills installed successfully.")
	return nil
}

func (s SkillInstallService) runNonInteractiveContainer(containerArgs []string) error {
	exitStatus, capturedStdout, capturedStderr, runErr := docker.DockerCapture(containerArgs)
	if runErr != nil {
		return runErr
	}
	if exitStatus != 0 {
		trimmedError := strings.TrimSpace(capturedStderr)
		if trimmedError == "" {
			trimmedError = strings.TrimSpace(capturedStdout)
		}
		if trimmedError == "" {
			return fmt.Errorf("skill installer failed with exit status %d", exitStatus)
		}
		return fmt.Errorf("skill installer failed (status %d): %s", exitStatus, trimmedError)
	}
	trimmedOutput := strings.TrimSpace(capturedStdout)
	if trimmedOutput != "" {
		s.Report.Info("%s", trimmedOutput)
	}
	s.Report.Success("✓ Agent skills installed successfully.")
	return nil
}
