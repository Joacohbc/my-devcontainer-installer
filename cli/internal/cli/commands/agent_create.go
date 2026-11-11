package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
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
the build.`,
		Example: `  # A Node project with a database
  devcontainer-cli agent create --modules nodejs,pnpm --services postgres

  # Pull a prebuilt image instead of building one
  devcontainer-cli agent create --mode profiles --profile nodejs

  # Combine a language profile with an add-on profile
  devcontainer-cli agent create --profile nodejs,my-tools --ports 3000:3000

  # Generate and build, but leave the stack down
  devcontainer-cli agent create --modules golang --no-up`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PreRunE:      agentCreateDefaults,
		RunE:         runAgentCreate,
	}
	addAgentCreateFlags(cmd)
	cmd.Flags().Bool(flagNoUp, false, "Generate and build only; leave the containers stopped")
	return cmd
}

func addAgentCreateFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String(flagMode, "", "Build mode: custom (default, full local build), profiles (pull a prebuilt image), or image (use a reusable image)")
	f.String(flagRegistry, "", "Container registry prefix for remote images (overrides global)")
	f.String(flagModules, "", "Comma-separated dockerfile modules (e.g. nodejs,golang,rust)")
	f.String(flagServices, "", "Comma-separated compose services (e.g. mongo,postgres,redis)")
	f.String(flagImage, "", "Image to run under mode=image: an image definition name (see 'image list'; a build definition must be built first, a remote one is pulled and verified) or a registry reference. Ignored for mode=custom, whose image is named by origin and content: devcontainer-cli/<profiles>-<hash>")
	f.String(flagWorkspace, "", "Workspace name (default: current dir name)")
	f.String(flagPorts, "", "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432); bound to 127.0.0.1 unless an IP is given; 'none' clears them")
	f.String(flagVolumes, "", "Extra volume mounts on the devcontainer (e.g. myvol:/data,./cache:/cache); 'none' clears them")
	f.String(flagForwardPorts, "", "Ports 'access forward' tunnels when called with no argument (e.g. 3000,8080:80, reverse:5432 for a host port reachable inside the container); 'none' clears them")
	f.Bool(flagSharedConfig, true, "Mount the global shared AI/dev tool config volume (devcontainer-shared-config) so logins/sessions persist across containers; --shared-config=false to opt out")
	f.String(flagProfile, "", "Apply profile(s): comma-separated module bundles for mode=custom (e.g. 'nodejs,my-tools'), or a single pull target for mode=profiles (must be [remote]-tagged). See 'profile list'.")
	f.StringArray(flagScript, nil, "Custom script to add, as <path>[:build|start|service|manual] (default build); repeatable. build bakes it into the image, start runs once per container, service runs as background service on every start, manual only copies it to ~/post-script/")
	f.String(flagSkills, "", "Comma-separated agent skills installed into the project workspace; implies the nodejs module. See 'config skill list' (built-in + your own)")
	f.String(flagSkillsMode, "", "How the project's agent skills get installed: manual (default — you run 'install-skills') or auto (on every container start, writing into the workspace unprompted)")

	completeProfileFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, profileIDs(false)), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagProfile, completeProfileFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagSkills, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.AgentSkillIDs(domain.SkillDirs()...)), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc(flagSkillsMode, staticCompletion(string(types.SkillModeAuto), string(types.SkillModeManual)))
	_ = cmd.RegisterFlagCompletionFunc(flagMode, staticCompletion(string(types.BuildModeCustom), string(types.BuildModeProfiles), string(types.BuildModeImage)))
	_ = cmd.RegisterFlagCompletionFunc(flagModules, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ModuleIDs()), cobra.ShellCompDirectiveNoFileComp
	})
	completeServiceFunc := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ServiceIDs()), cobra.ShellCompDirectiveNoFileComp
	}
	_ = cmd.RegisterFlagCompletionFunc(flagServices, completeServiceFunc)
	_ = cmd.RegisterFlagCompletionFunc(flagImage, completeImageNames)
}

// agentCreateDefaults applies what makes this "create" rather than "generate":
// no prompts; force and build are handled internally.
func agentCreateDefaults(cmd *cobra.Command, _ []string) error {
	return agentDefaults(cmd, map[string]string{
		flagNoInteractive: "true",
	})
}

func runAgentCreate(cmd *cobra.Command, args []string) error {
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
