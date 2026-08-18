package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// flagNoUp opts out of the 'up' that otherwise closes an agent create.
const flagNoUp = "no-up"

func newAgentCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Generate, build and start a devcontainer for the current directory",
		Long: `devcontainer-cli agent create — take the current directory from nothing to a
running devcontainer in one command: generate the Dockerfile, docker-compose.yml
and .env, build (or pull) the image, then start the stack.

It is the wizard's job done from flags. Where a bare 'devcontainer-cli' would
open the wizard and ask, this never prompts: modules, services and ports come
from the flags, existing generated files are overwritten, and the build runs
without asking. Re-run it with different flags to change an existing project.

Run 'agent cli-info' first for the module, service, profile and skill ids this
accepts.

The one thing it adds over generation is the start: the containers are running
when it returns, so 'agent exec' works immediately. Pass --no-up to stop after
the build.

For throwaway isolated tasks with no project directory or configuration files,
pass --temporal together with --profile to spawn an ephemeral container.`,
		Example: `  # A Node project with a database
  devcontainer-cli agent create --with nodejs,pnpm --service postgres

  # Pull a prebuilt image instead of building one
  devcontainer-cli agent create --mode profiles --profile nodejs

  # Start from a profile and publish a port
  devcontainer-cli agent create --profile scraper --ports 3000:3000

  # Generate and build, but leave the stack down
  devcontainer-cli agent create --with golang --no-up

  # Run a throwaway temporal container from a prebuilt profile
  devcontainer-cli agent create --temporal --profile python --name my-task`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PreRunE:      agentCreateDefaults,
		RunE:         runAgentCreate,
	}
	addAgentCreateFlags(cmd)
	cmd.Flags().Bool(flagNoUp, false, "Generate and build only; leave the containers stopped")
	cmd.Flags().Bool("temporal", false, "Run an ephemeral container without project files")
	cmd.Flags().String("name", "", "Container name for temporal run (default: dc-<profile>)")
	cmd.Flags().Bool("expose-all", false, "Publish ports on all interfaces for temporal run")
	cmd.Flags().Bool("copy-ai-scripts", false, "Copy AI scripts for temporal run")
	return cmd
}

func addAgentCreateFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String(flagMode, "", "Build mode: custom (default, full local build) or profiles (pull a prebuilt image)")
	f.String(flagRegistry, "", "Container registry prefix for remote images (overrides global)")
	f.String(flagWith, "", "Comma-separated dockerfile modules (e.g. nodejs,golang,rust)")
	f.String(flagService, "", "Comma-separated compose services (e.g. mongo,postgres,redis)")
	f.String(flagServices, "", "Alias for --service")
	f.String(flagImage, "", "Image name (default: derived from fingerprint for mode=custom)")
	f.String(flagWorkspace, "", "Workspace name (default: current dir name)")
	f.String(flagPorts, "", "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them")
	f.String(flagVolumes, "", "Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them")
	f.String(flagForwardPorts, "", "Ports 'port-forward' tunnels when called with no argument (e.g. 3000,8080:80); 'none' clears them")
	f.Bool(flagSharedConfig, true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	f.String(flagProfile, "", "Apply a profile: a module bundle for mode=custom (any profile, e.g. 'scraper'), or the pull target for mode=profiles (must be [remote]-tagged). See 'config profile list'.")
	f.StringArray(flagScript, nil, "Custom script to add, as <path>[:build|start|manual] (default build); repeatable. build bakes it into the image, start runs it once per container, manual only copies it to ~/post-script/")
	f.String(flagSkill, "", "Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own)")
	f.String(flagSkillsMode, "", "How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted)")
	f.Bool(flagNoInteractive, false, "Fail if any value is missing instead of prompting")
	f.Bool(flagForce, false, "Overwrite existing files without prompting")
	f.Bool(flagBuild, false, "Run 'docker compose build/pull' after generating")
	f.Bool(flagNoBuild, false, "Skip the build/pull step after generating")

	completeProfileFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileIDs(false), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagProfile, completeProfileFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagSkill, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.AgentSkillIDs(domain.SkillDirs()...)), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagSkillsMode, staticCompletion(string(types.SkillModeAuto), string(types.SkillModeManual)))
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

// agentCreateDefaults applies what makes this "create" rather than "generate":
// no prompts, no overwrite question, and a build that just runs.
func agentCreateDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
		flagForce:         "true",
		flagBuild:         "true",
	})
}

func runAgentCreate(cmd *cobra.Command, args []string) error {
	if temporal, _ := cmd.Flags().GetBool("temporal"); temporal {
		return runAgentTemporal(cmd, args)
	}

	if err := runGenerate(cmd, args); err != nil {
		return err
	}
	if noUp, _ := cmd.Flags().GetBool(flagNoUp); noUp {
		return nil
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	composeFile, err := resolveProjectComposeFile(cwd)
	if err != nil {
		return err
	}

	// The image was already built (or pulled) by the generate above, so this
	// only has to create and start the containers.
	if err := (service.LifecycleService{Report: console}).Up(composeFile, resolveWorkspace(cwd), false); err != nil {
		return err
	}
	console.Done()
	return nil
}

func runAgentTemporal(cmd *cobra.Command, _ []string) error {
	svc := service.RunService{Report: console}

	variant, _ := cmd.Flags().GetString(flagProfile)
	if variant == "" {
		return fmt.Errorf("--profile is required for temporal containers")
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = "dc-" + variant
	}

	rawVolumes, _ := cmd.Flags().GetString(flagVolumes)
	volumes := parseClearableCSV(cmd.Flags().GetString, flagVolumes)
	if rawVolumes == "none" {
		volumes = nil
	}

	rawPorts, _ := cmd.Flags().GetString(flagPorts)
	ports := parseClearableCSV(cmd.Flags().GetString, flagPorts)
	if rawPorts == "none" {
		ports = nil
	}

	exposeAll, _ := cmd.Flags().GetBool("expose-all")
	sharedConfig, _ := cmd.Flags().GetBool(flagSharedConfig)
	copyScripts, _ := cmd.Flags().GetBool("copy-ai-scripts")

	registry, _ := cmd.Flags().GetString(flagRegistry)
	image := domain.ResolveRemoteImage(variant, domain.ResolveRegistry(registry, ""))

	if err := svc.Run(service.QuickRunSpec{
		Variant:       variant,
		ContainerName: name,
		Image:         image,
		Volumes:       volumes,
		Ports:         ports,
		ExposeAll:     exposeAll,
		SharedConfig:  sharedConfig,
	}); err != nil {
		return err
	}

	if copyScripts {
		if err := svc.CopyAIScripts(name, assets.CopyableNames()); err != nil {
			return err
		}
	}

	console.NewLine()
	printRunNextSteps(name)
	return nil
}
