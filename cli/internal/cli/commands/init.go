package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newInitCommand()) }

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [repo-url|dir] [target-dir] [flags]",
		Short: "Initialize a devcontainer by detecting project stack or cloning a repository",
		Long: `devcontainer-cli init — inspects project files to automatically detect language
stacks, runtimes, package managers, and tools (JS/TS, Go, Python, Java/Maven/Gradle,
PHP, Rust, C/C++, SQLite, GitHub Actions, Docker-in-Docker, and Agent Skills),
generates the devcontainer configuration, builds the image, starts the container stack,
and installs agent skills if a skill lockfile is detected.

Optionally accepts a Git repository URL to clone first.`,
		Example: `  # Initialize devcontainer in current directory with automatic stack detection
  devcontainer-cli init

  # Clone a remote git repository and initialize its devcontainer environment
  devcontainer-cli init https://github.com/user/my-repo.git

  # Clone into a specific target folder
  devcontainer-cli init git@github.com:user/project.git ./my-project

  # Initialize a specific directory without starting containers immediately
  devcontainer-cli init ./backend --no-up

  # Initialize with extra modules and compose services
  devcontainer-cli init --with redis-client --service redis`,
		Args:         cobra.MaximumNArgs(2),
		SilenceUsage: true,
		RunE:         runInit,
	}

	addInitFlags(cmd)
	return cmd
}

func addInitFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String(flagMode, "", "Build mode: custom (default) or profiles (pull prebuilt image)")
	f.String(flagRegistry, "", "Container registry prefix for remote images")
	f.String(flagWith, "", "Additional dockerfile modules (e.g. nodejs,golang,rust)")
	f.String(flagService, "", "Additional compose services (e.g. mongo,postgres,redis)")
	f.String(flagServices, "", "Alias for --service")
	f.String(flagWorkspace, "", "Workspace name (default: directory name or repo name)")
	f.String(flagPorts, "", "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432)")
	f.String(flagVolumes, "", "Extra volume mounts on the devcontainer")
	f.String(flagForwardPorts, "", "Ports 'port-forward' tunnels by default (a 'reverse:' prefix tunnels a host port into the container)")
	f.Bool(flagSharedConfig, true, "Mount global shared tool config volume")
	f.String(flagProfile, "", "Apply a profile bundle")
	f.Bool(flagNoUp, false, "Generate and build only; leave containers stopped")
	f.Bool(flagNoBuild, false, "Skip image build step after generating")
	f.Bool(flagNoInteractive, false, "Non-interactive mode")
	f.Bool(flagNonInteractive, false, "Alias for --no-interactive")
	f.Bool(flagForce, false, "Overwrite existing files without prompting")

	completeProfileFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileIDs(false), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagProfile, completeProfileFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagMode, staticCompletion(string(types.BuildModeCustom), string(types.BuildModeProfiles)))
	_ = cmd.RegisterFlagCompletionFunc(flagWith, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ModuleIDs()), cobra.ShellCompDirectiveNoFileComp
	})
	completeServiceFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ServiceIDs()), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagService, completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagServices, completeServiceFunc)
}

func runInit(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()
	noI, _ := f.GetBool(flagNoInteractive)
	nonI, _ := f.GetBool(flagNonInteractive)
	force, _ := f.GetBool(flagForce)
	noUp, _ := f.GetBool(flagNoUp)
	noBuild, _ := f.GetBool(flagNoBuild)

	opts := service.InitOptions{
		Interactive: !(noI || nonI),
		Force:       force,
		NoUp:        noUp,
		NoBuild:     noBuild,
	}

	if len(args) > 0 {
		firstArg := args[0]
		if domain.IsGitRepoURL(firstArg) {
			opts.RepoURL = firstArg
			if len(args) > 1 {
				opts.TargetDir = args[1]
			}
		} else {
			opts.TargetDir = firstArg
		}
	}

	opts.Workspace, _ = f.GetString(flagWorkspace)
	opts.Mode, _ = f.GetString(flagMode)
	opts.Registry, _ = f.GetString(flagRegistry)
	opts.Profile, _ = f.GetString(flagProfile)

	if f.Changed(flagWith) {
		w, _ := f.GetString(flagWith)
		opts.WithModules = splitCSV(w)
	}
	if f.Changed(flagService) || f.Changed(flagServices) {
		s, _ := f.GetString(flagService)
		if s == "" {
			s, _ = f.GetString(flagServices)
		}
		opts.Services = splitCSV(s)
	}
	if f.Changed(flagPorts) {
		opts.Ports = parsePortsFlag(f.GetString)
	}
	if f.Changed(flagVolumes) {
		opts.Volumes = parseClearableCSV(f.GetString, flagVolumes)
	}
	if f.Changed(flagForwardPorts) {
		opts.ForwardPorts = parseClearableCSV(f.GetString, flagForwardPorts)
	}
	if f.Changed(flagSharedConfig) {
		v, _ := f.GetBool(flagSharedConfig)
		opts.SharedConfig = &v
	}

	initSvc := service.InitService{
		Report: console,
		Prompt: console,
	}

	console.Header("\nDevContainer Stack Initializer")
	console.NewLine()

	_, err := initSvc.Init(opts)
	if err != nil {
		return err
	}

	console.Done()
	return nil
}
