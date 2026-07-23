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
		Short: "Open a real SSH session into the devcontainer, configuring access first if needed",
		Long: `devcontainer-cli ssh — connect to the project's devcontainer over SSH.

Unlike 'shell' (a 'docker exec' wrapper), this opens a genuine SSH session, so
it exercises the same path a human or tool connecting via 'ssh <alias>' would.
If no managed SSH alias exists yet for the workspace it runs the same setup
'setup-ssh' does (generate/install the shared key, append a ~/.ssh/config Host
block) automatically before connecting — no separate 'setup-ssh' call needed.

Always targets the project's own devcontainer service, the same one
'shell'/'setup-ssh' resolve by default; it has no --container escape hatch to
other containers or jump hosts — use 'port-forward' or 'shell -c' for those.

With --forward (or by answering yes to the interactive prompt) it also opens
SSH tunnels for the given ports alongside the session, torn down automatically
when the session ends.`,
		Example: `  # Connect (runs setup-ssh automatically the first time)
  devcontainer-cli ssh

  # Also forward ports 3000 and 8080 for the session
  devcontainer-cli ssh --forward --ports 3000,8080:80

  # Run a one-off command over SSH instead of an interactive shell
  devcontainer-cli ssh -- go version`,
		RunE: runSsh,
	}
	addWorkspaceFlag(cmd)
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	cmd.Flags().Bool("forward", false, "Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted)")
	cmd.Flags().String("ports", "", "Ports to forward, e.g. '3000,8080:80' (implies --forward; skips the prompt)")
	return cmd
}

func runSsh(cmd *cobra.Command, args []string) error {
	wsFlag := workspaceFlag(cmd)
	interactive := interactiveFlag(cmd)
	assumeYes := yesFlag(cmd) || !interactive

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	workspace := resolveWorkspace(cwd, wsFlag)
	containerName, err := resolveDevcontainerContainer(cwd, wsFlag)
	if err != nil {
		return err
	}

	ssh := service.SshService{Report: console}
	alias, ok, err := ssh.ManagedAlias(sshdefaults.KindWorkspace, workspace)
	if err != nil {
		return err
	}
	if !ok {
		console.Info("No SSH access configured yet for '%s'; setting it up...", workspace)
		if err := checkPrereqs(sshdefaults.ModeLocal); err != nil {
			return err
		}
		f := &setupSshFlags{
			alias:       workspace,
			key:         domain.ResolveSSHKeyPath(""),
			container:   containerName,
			service:     sshdefaults.ServiceName,
			composeFile: relativeComposeFile(workspace),
			user:        sshdefaults.User,
			assumeYes:   assumeYes,
		}
		if err := performSetupSsh(ssh, f, sshdefaults.ModeLocal, workspace); err != nil {
			return err
		}
		alias = f.alias
	}

	tunnels, err := resolveSshTunnels(cmd, alias, containerName, interactive)
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
