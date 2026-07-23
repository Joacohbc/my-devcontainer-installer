package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newSshCommand()) }

func newSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [flags] [-- command args...]",
		Short: "Open a real SSH session into a container, configuring access first if needed",
		Long: `devcontainer-cli ssh — connect to a container over SSH.

Unlike 'shell' (a 'docker exec' wrapper), this opens a genuine SSH session, so
it exercises the same path a human or tool connecting via 'ssh <alias>' would.
If no managed SSH alias exists yet for the target it runs the same setup
'setup-ssh' does (generate/install the shared key, append a ~/.ssh/config Host
block) automatically before connecting — no separate 'setup-ssh' call needed.

By default it targets the project's own devcontainer service (the same one
'shell'/'setup-ssh' resolve without --container). Pass --container to reach any
other CLI-managed container instead (loose mode, same as 'setup-ssh
--container'); there's no jump-host support for unmanaged containers — use
'port-forward' for that.

With --forward (or by answering yes to the interactive prompt) it also opens
SSH tunnels for the given ports alongside the session, torn down automatically
when the session ends.

Closing an interactive session is always treated as success — even if the last
command in your shell exited non-zero — so only a real connection failure
reports an error. With an explicit command after '--', the command's exit code
is propagated instead, which makes 'ssh' usable in scripts.`,
		Example: `  # Connect to the project's devcontainer (runs setup-ssh automatically the first time)
  devcontainer-cli ssh

  # Connect to another managed container (e.g. a sidecar service)
  devcontainer-cli ssh -c myws-postgres

  # Also forward ports 3000 and 8080 for the session
  devcontainer-cli ssh --forward --ports 3000,8080:80

  # Run a one-off command over SSH instead of an interactive shell
  devcontainer-cli ssh -- go version`,
		RunE: runSsh,
	}
	addWorkspaceFlag(cmd)
	addContainerFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("forward", false, "Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted)")
	cmd.Flags().String("ports", "", "Ports to forward, e.g. '3000,8080:80' (implies --forward; skips the prompt)")
	return cmd
}

// sshTarget is what 'ssh' resolves the command line down to: which container
// to reach, and the managed-block kind/ref used both to look up an existing
// SSH alias and, if none exists, to tag the block setup-ssh creates.
type sshTarget struct {
	containerName     string
	containerExplicit bool // loose --container mode, mirroring setup-ssh's
	markerKind        sshdefaults.Kind
	markerRef         string // workspace name, or the container name in loose mode
}

// resolveSshTarget picks workspace mode (the project's own devcontainer,
// looked up by workspace name) or loose container mode (--container, like
// 'setup-ssh --container') based on whether --container was passed explicitly.
func resolveSshTarget(cwd, wsFlag, containerFlag string, containerExplicit bool) (sshTarget, error) {
	if containerExplicit {
		return sshTarget{
			containerName:     containerFlag,
			containerExplicit: true,
			markerKind:        sshdefaults.KindContainer,
			markerRef:         containerFlag,
		}, nil
	}
	workspace := resolveWorkspace(cwd, wsFlag)
	containerName, err := resolveDevcontainerContainer(cwd, wsFlag)
	if err != nil {
		return sshTarget{}, err
	}
	return sshTarget{
		containerName: containerName,
		markerKind:    sshdefaults.KindWorkspace,
		markerRef:     workspace,
	}, nil
}

// bootstrapSetupSshFlags builds the setupSshFlags 'ssh' hands to
// performSetupSsh when target has no managed alias yet, plus the workspace
// label to log (setup-ssh's own "(none)" convention in loose container mode).
func bootstrapSetupSshFlags(target sshTarget, assumeYes bool) (*setupSshFlags, string) {
	f := &setupSshFlags{
		key:               domain.ResolveSSHKeyPath(""),
		container:         target.containerName,
		containerExplicit: target.containerExplicit,
		service:           sshdefaults.ServiceName,
		user:              sshdefaults.User,
		assumeYes:         assumeYes,
	}
	if target.containerExplicit {
		f.alias = target.containerName
		return f, "(none)"
	}
	workspace := target.markerRef
	f.alias = workspace
	f.composeFile = relativeComposeFile(workspace)
	return f, workspace
}

func runSsh(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	interactive := interactiveFlag(cmd)
	assumeYes := yesFlag(cmd) || !interactive

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	containerFlag, _ := cmd.Flags().GetString("container")
	containerExplicit := cmd.Flags().Changed("container") && containerFlag != ""

	target, err := resolveSshTarget(cwd, wsFlag, containerFlag, containerExplicit)
	if err != nil {
		return err
	}

	ssh := service.SshService{Report: console}
	alias, ok, err := ssh.ManagedAlias(target.markerKind, target.markerRef)
	if err != nil {
		return err
	}
	if !ok {
		console.Info("No SSH access configured yet for '%s'; setting it up...", target.markerRef)
		if err := checkPrereqs(sshdefaults.ModeLocal); err != nil {
			return err
		}
		f, workspace := bootstrapSetupSshFlags(target, assumeYes)
		if err := performSetupSsh(ssh, f, sshdefaults.ModeLocal, workspace); err != nil {
			return err
		}
		alias = f.alias
	}

	tunnels, err := resolveSshTunnels(cmd, alias, target.containerName, interactive)
	if err != nil {
		return err
	}

	if len(tunnels) > 0 {
		printTunnelPlan(tunnels)
		stop, err := (service.PortForwardService{Report: console}).OpenTunnelsDetached(tunnels)
		if err != nil {
			return err
		}
		defer stop()
		console.Success("Port forwarding active in the background; connecting...\n")
	}

	return ssh.Connect(alias, args)
}

// resolveSshTunnels decides which ports (if any) to forward alongside the SSH
// session. --ports always implies forwarding and skips the prompts entirely;
// --forward without --ports still asks which ports (or requires --ports in
// non-interactive mode); with neither flag it asks whether to forward at all,
// but only when interactive — non-interactive runs default to no forwarding.
func resolveSshTunnels(cmd *cobra.Command, alias, containerName string, interactive bool) ([]service.Tunnel, error) {
	portsFlag, _ := cmd.Flags().GetString("ports")
	forward, _ := cmd.Flags().GetBool("forward")

	if portsFlag == "" && !forward {
		if !interactive {
			return nil, nil
		}
		want, err := console.ConfirmDefault("Also forward ports from this container over SSH?", false)
		if err != nil {
			return nil, err
		}
		if !want {
			return nil, nil
		}
	}

	if portsFlag == "" {
		if !interactive {
			return nil, fmt.Errorf("--ports is required with --forward in non-interactive mode")
		}
		var err error
		portsFlag, err = console.AskDefault("Ports to forward (e.g. 3000, 8080:80):", "", func(v string) error {
			_, e := parsePortsList(v)
			return e
		})
		if err != nil {
			return nil, err
		}
	}

	return buildSshTunnels(portsFlag, alias, containerName)
}

// buildSshTunnels parses a comma-separated ports spec into Tunnels reaching
// containerName's own loopback through alias.
func buildSshTunnels(portsFlag, alias, containerName string) ([]service.Tunnel, error) {
	pairs, err := parsePortsList(portsFlag)
	if err != nil {
		return nil, err
	}
	tunnels := make([]service.Tunnel, len(pairs))
	for i, p := range pairs {
		tunnels[i] = service.Tunnel{
			LocalPort:      p.localPort,
			ContainerPort:  p.containerPort,
			TargetHost:     "localhost",
			Alias:          alias,
			ContainerName:  containerName,
			IsDevcontainer: true,
		}
	}
	return tunnels, nil
}
