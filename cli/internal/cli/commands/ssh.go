package commands

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// sshNoWorkspace is the placeholder logged in place of a workspace name when
// --container bypasses workspace resolution entirely.
const sshNoWorkspace = "(none)"

func init() { register(newSshCommand()) }

func newSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [flags] [-- command args...]",
		Short: "Open a real SSH session into the devcontainer, configuring access first if needed",
		Long: `devcontainer-cli ssh — connect to the project's devcontainer over SSH.

Unlike 'shell' (a 'docker exec' wrapper), this opens a genuine SSH session, so
it exercises the same path a human or tool connecting via 'ssh <alias>' would.
If no managed SSH alias exists yet for the target it runs the same setup
'setup-ssh' does (generate/install the shared key, append a ~/.ssh/config Host
block) automatically before connecting — no separate 'setup-ssh' call needed.

By default it targets the project's own devcontainer service, the same one
'shell'/'setup-ssh' resolve by default. Pass --container to connect to any other
managed container instead (loose mode, like 'setup-ssh --container'); the
managed alias is then keyed by that container's name rather than the workspace.

Before connecting it re-pins the container's current SSH host keys (read through
docker) in the CLI-managed known_hosts, so a rebuilt image — new host keys, same
container IP — never aborts the session with "REMOTE HOST IDENTIFICATION HAS
CHANGED". Host blocks written by older versions are upgraded to that scheme on
first use, leaving your global ~/.ssh/known_hosts alone.

With --forward (or by answering yes to the interactive prompt) it also opens
SSH tunnels for the given ports alongside the session, torn down automatically
when the session ends.`,
		Example: `  # Connect (runs setup-ssh automatically the first time)
  devcontainer-cli ssh

  # Connect to a specific container instead of the project's own devcontainer
  devcontainer-cli ssh --container dc-ssh

  # Also forward ports 3000 and 8080 for the session
  devcontainer-cli ssh --forward --ports 3000,8080:80

  # Run a one-off command over SSH instead of an interactive shell
  devcontainer-cli ssh -- go version`,
		RunE: runSsh,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	cmd.Flags().Bool("forward", false, "Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted)")
	cmd.Flags().String("ports", "", "Ports to forward, e.g. '3000,8080:80' (implies --forward; skips the prompt)")
	return cmd
}

func runSsh(cmd *cobra.Command, args []string) error {
	interactive := interactiveFlag(cmd)
	assumeYes := yesFlag(cmd) || !interactive

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	containerExplicit := cmd.Flags().Changed("container")
	var workspace, containerName string
	kind := sshdefaults.KindWorkspace
	ref := ""

	if containerExplicit {
		containerName, _ = cmd.Flags().GetString("container")
		kind = sshdefaults.KindContainer
		ref = containerName
	} else {
		workspace = resolveWorkspace(cwd)
		containerName, err = resolveDevcontainerContainer(cwd)
		if err != nil {
			return err
		}
		ref = workspace
	}

	ssh := service.SshService{Report: console}
	alias, ok, err := ssh.ManagedAlias(kind, ref)
	if err != nil {
		return err
	}
	if !ok {
		console.Info("No SSH access configured yet for '%s'; setting it up...", ref)
		if err := checkPrereqs(sshdefaults.ModeLocal); err != nil {
			return err
		}
		f := &setupSshFlags{
			alias:             ref,
			key:               domain.ResolveSSHKeyPath(""),
			knownHosts:        domain.ManagedKnownHostsPath(),
			container:         containerName,
			containerExplicit: containerExplicit,
			service:           sshdefaults.ServiceName,
			user:              sshdefaults.User,
			assumeYes:         assumeYes,
		}
		logWorkspace := workspace
		if !containerExplicit {
			f.composeFile = relativeComposeFile(workspace)
		} else {
			logWorkspace = sshNoWorkspace
		}
		if err := performSetupSsh(ssh, f, sshdefaults.ModeLocal, logWorkspace); err != nil {
			return err
		}
		alias = f.alias
	} else {
		refreshHostKey(ssh, alias, containerName)
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

// refreshHostKey re-pins the container's current ssh host keys before reusing an
// already configured alias. A container regenerates its host keys whenever its
// image is rebuilt while keeping the same IP, which otherwise makes ssh abort
// with "REMOTE HOST IDENTIFICATION HAS CHANGED"; asking docker for the keys the
// container has right now turns that failure into a silent re-pin. Blocks
// written before host-key pinning existed are upgraded first so their keys live
// in the CLI-managed known_hosts instead of the user's global one.
//
// Every step is best-effort: on failure the session still opens, ssh just falls
// back to whatever the block already said.
func refreshHostKey(ssh service.SshService, alias, containerName string) {
	knownHosts := domain.ManagedKnownHostsPath()
	updated, err := ssh.EnsureHostKeyPinning(alias, knownHosts)
	if err != nil {
		console.Debug("could not update host-key options for '%s': %v", alias, err)
		return
	}
	if updated {
		console.Info("Host '%s' now verifies host keys against %s", alias, knownHosts)
	}

	host, err := ssh.AliasHostName(alias)
	if err != nil || host == "" {
		return // no HostName (e.g. a ProxyCommand block): nothing local to pin
	}
	if err := ssh.PinContainerHostKeys(containerName, host); err != nil {
		console.Debug("could not pin host key for %s: %v", host, err)
	}
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
