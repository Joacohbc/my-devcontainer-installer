package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/git"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

type InitOptions struct {
	TargetDir    string
	RepoURL      string
	Workspace    string
	WithModules  []string
	Services     []string
	Ports        []string
	Volumes      []string
	ForwardPorts []string
	SharedConfig *bool
	Mode         string
	Registry     string
	Profile      string
	Skills       []types.SkillID
	SkillsMode   string
	Scripts      []types.CustomScript
	NoUp         bool
	NoBuild      bool
	Interactive  bool
	Force        bool
}

type InitService struct {
	Report Reporter
	Prompt Prompter
}

func (s InitService) Init(opts InitOptions) (*types.DevcontainerConfig, error) {
	targetDir, err := s.resolveTargetDirectory(opts)
	if err != nil {
		return nil, err
	}

	stack, err := domain.DetectStack(targetDir)
	if err != nil {
		return nil, fmt.Errorf("detecting project stack in %s: %w", targetDir, err)
	}

	s.reportDetectedStack(stack)

	config := domain.DefaultConfig(targetDir)
	s.applyWorkspaceName(config, targetDir, opts)
	s.applyDetectedAndUserModules(config, stack, opts)
	s.applyDetectedAndUserServices(config, stack, opts)
	s.applyConfigOptions(config, opts)

	paths := project.ProjectPaths(targetDir, config.Workspace)
	genSvc := GenerateService{Report: s.Report}

	copyContents, _, err := genSvc.PrepareBuildDir(config, paths)
	if err != nil {
		return nil, err
	}

	plan, err := genSvc.Plan(config, copyContents)
	if err != nil {
		return nil, err
	}
	config.Fingerprint = plan.Fingerprint
	config.Image = plan.Image

	if err := genSvc.WriteFiles(paths, &plan, config); err != nil {
		return nil, err
	}

	if err := domain.SaveConfig(config, targetDir); err != nil {
		return nil, err
	}
	s.Report.Success("Saved devcontainer.config.json")

	if types.SharedConfigEnabled(config) {
		if err := genSvc.EnsureSharedConfigVolume(); err != nil {
			s.Report.Warn("Could not ensure shared-config volume: %v", err)
		}
	}

	domain.RecordProject(targetDir, config, "")

	isRemote := config.Mode == types.BuildModeProfiles
	if !opts.NoBuild {
		if err := genSvc.Build(paths.ComposeFile, isRemote); err != nil {
			return nil, err
		}
	}

	if !opts.NoUp && !opts.NoBuild {
		if err := (LifecycleService{Report: s.Report}).Up(paths.ComposeFile, config.Workspace, false); err != nil {
			return nil, err
		}

		if stack.HasSkillsLock {
			s.InstallSkills(config.Workspace)
		}
	}

	return config, nil
}

func (s InitService) resolveTargetDirectory(opts InitOptions) (string, error) {
	target := opts.TargetDir
	if opts.RepoURL != "" {
		if target == "" {
			target = domain.ExtractRepoName(opts.RepoURL)
		}
		s.Report.Info("Cloning repository %s into %s...", opts.RepoURL, target)
		if err := git.Clone(opts.RepoURL, target); err != nil {
			return "", err
		}
		s.Report.Success("Cloned repository successfully.")
	}
	if target == "" {
		target = "."
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path for %s: %w", target, err)
	}
	if _, statErr := os.Stat(absPath); statErr != nil {
		return "", fmt.Errorf("target directory does not exist: %s", absPath)
	}
	return absPath, nil
}

func (s InitService) reportDetectedStack(stack domain.DetectedStack) {
	if len(stack.DetectedFiles) > 0 {
		s.Report.Info("Detected stack files: %s", strings.Join(stack.DetectedFiles, ", "))
	}
	if len(stack.Modules) > 0 {
		var moduleNames []string
		for _, m := range stack.Modules {
			moduleNames = append(moduleNames, string(m))
		}
		s.Report.Success("Detected modules: %s", strings.Join(moduleNames, ", "))
	}
	if stack.HasSkillsLock {
		s.Report.Info("Detected skill lockfile (agent skills will be installed on start).")
	}
}

func (s InitService) applyWorkspaceName(config *types.DevcontainerConfig, targetDir string, opts InitOptions) {
	if opts.Workspace != "" {
		config.Workspace = opts.Workspace
		return
	}
	if opts.RepoURL != "" {
		config.Workspace = domain.ExtractRepoName(opts.RepoURL)
		return
	}
	config.Workspace = domain.ResolveWorkspace(targetDir, config)
}

func (s InitService) applyDetectedAndUserModules(config *types.DevcontainerConfig, stack domain.DetectedStack, opts InitOptions) {
	selected := make(map[types.ModuleID]bool)
	for _, m := range stack.Modules {
		selected[domain.NormalizeModuleID(string(m))] = true
	}
	for _, modName := range opts.WithModules {
		selected[domain.NormalizeModuleID(modName)] = true
	}
	if stack.HasSkillsLock {
		selected[types.ModuleNodejs] = true
	}

	var orderedModules []types.SelectedModule
	for _, mod := range catalog.DockerfileModules {
		if selected[mod.ID] {
			orderedModules = append(orderedModules, types.SelectedModule{ID: mod.ID})
		}
	}
	config.Dockerfile.Modules = orderedModules
}

func (s InitService) applyDetectedAndUserServices(config *types.DevcontainerConfig, stack domain.DetectedStack, opts InitOptions) {
	selected := make(map[types.ServiceID]bool)
	for _, s := range stack.Services {
		selected[domain.NormalizeServiceID(string(s))] = true
	}
	for _, s := range opts.Services {
		selected[domain.NormalizeServiceID(s)] = true
	}

	var orderedServices []types.SelectedService
	for _, svc := range catalog.ComposeServices {
		if selected[svc.ID] {
			orderedServices = append(orderedServices, types.SelectedService{ID: svc.ID})
		}
	}
	config.Compose.Services = orderedServices
}

func (s InitService) applyConfigOptions(config *types.DevcontainerConfig, opts InitOptions) {
	if opts.Mode != "" {
		config.Mode = types.BuildMode(opts.Mode)
	}
	if opts.Profile != "" {
		p, ok := catalog.Resolve(opts.Profile, domain.ProfileDirs()...)
		if ok {
			config.Dockerfile.Scripts = append(config.Dockerfile.Scripts, p.Scripts...)
			config.Compose.Ports = append(config.Compose.Ports, p.Ports...)
			config.ForwardPorts = append(config.ForwardPorts, p.ForwardPorts...)
		}
	}
	if len(opts.Ports) > 0 {
		config.Compose.Ports = append(config.Compose.Ports, opts.Ports...)
	}
	if len(opts.Volumes) > 0 {
		config.Compose.Volumes = append(config.Compose.Volumes, opts.Volumes...)
	}
	if len(opts.ForwardPorts) > 0 {
		config.ForwardPorts = append(config.ForwardPorts, opts.ForwardPorts...)
	}
	if opts.SharedConfig != nil {
		config.Compose.SharedConfig = opts.SharedConfig
	}
	if len(opts.Scripts) > 0 {
		config.Dockerfile.Scripts = append(config.Dockerfile.Scripts, opts.Scripts...)
	}
	if len(opts.Skills) > 0 {
		config.Skills.Skills = opts.Skills
	}
	if opts.SkillsMode != "" {
		config.Skills.Mode = types.SkillMode(opts.SkillsMode)
	}
}

func (s InitService) InstallSkills(workspace string) {
	s.Report.Info("Installing agent skills from lockfile inside devcontainer...")
	containerName := workspace + "-" + sshdefaults.ServiceName
	inspectSvc := InspectService{Report: s.Report}
	workspaceMount := types.WorkspaceDir(workspace)

	installErr := inspectSvc.Shell(containerName, "devuser", "", workspaceMount, []string{"npx", "--yes", "skills", "experimental_install"}, true)
	if installErr != nil {
		s.Report.Warn("experimental_install encountered an issue, trying skills install: %v", installErr)
		if retryErr := inspectSvc.Shell(containerName, "devuser", "", workspaceMount, []string{"npx", "--yes", "skills", "install"}, true); retryErr != nil {
			s.Report.Warn("Could not install skills automatically: %v", retryErr)
			return
		}
	}
	s.Report.Success("Agent skills installed successfully.")
}
