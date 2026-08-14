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
		Long: `devcontainer-cli ssh — connect to the project's devcontainer over SSH, and
own the whole SSH lifecycle (this is where 'setup-ssh' used to live).

Unlike 'shell' (a 'docker exec' wrapper), this opens a genuine SSH session, so
it exercises the same path a human or tool connecting via 'ssh <alias>' would.
The first time, it configures access automatically (generate/install the shared
key, append a Host block to the CLI's own SSH config) before connecting — no
separate command needed. Pass --setup to force that configuration to run again
(rotate the key, repair the Host block) before connecting.

By default it targets the project's own devcontainer service, the same one
'shell' resolves. Pass --container to connect to any other managed container
instead (loose mode); the managed alias is then keyed by that container's name
rather than the workspace.

Pass --via USER@HOST (with --container) to reach a container on a different
Docker host through an existing SSH connection to it. Only the shared key's
PUBLIC half ever leaves this machine. --via only needs to be given once: the
alias remembers the jump target for future reconnects.

Pass --setup-external USER@HOST when this CLI runs ON the Docker host: instead of
connecting, it prints a self-contained snippet (containing the PRIVATE key) to
paste on the machine you connect FROM, which sets up a ProxyCommand jump there.

Before connecting it re-pins the container's current SSH host keys (read through
docker) in the CLI-managed known_hosts, so a rebuilt image — new host keys —
never aborts the session with "REMOTE HOST IDENTIFICATION HAS CHANGED". The
target is anchored by container name, not IP: a container that came back on a
different address has its Host block refreshed in place. It also rules out a
stale target deterministically before dialing (a container that no longer
exists is reported as a stale alias rather than a confusing SSH timeout), and at
setup time offers a real SSH probe — shown and confirmed first — to verify
end-to-end reachability.

With --forward (or by answering yes to the interactive prompt) it also opens
SSH tunnels for the given ports alongside the session, torn down automatically
when the session ends.`,
		Example: `  # Connect (configures access automatically the first time)
  devcontainer-cli ssh

  # Re-run the access setup, then connect
  devcontainer-cli ssh --setup

  # Connect to a specific container instead of the project's own devcontainer
  devcontainer-cli ssh --container dc-ssh

  # Connect through an existing SSH connection to a remote Docker host
  devcontainer-cli ssh --via me@docker-host --container dc-ssh

  # Run ON the Docker host: print the snippet to paste on the connecting machine
  devcontainer-cli ssh --setup-external me@docker-host

  # Also forward ports 3000 and 8080 for the session
  devcontainer-cli ssh --forward --ports 3000,8080:80

  # Run a one-off command over SSH instead of an interactive shell
  devcontainer-cli ssh -- go version`,
		RunE: runSsh,
	}
	addSshFlags(cmd)
	return cmd
}

// addSshFlags registers the flags runSsh reads. It is shared with the
// 'agent connect' facade, which reuses runSsh with agent-safe defaults.
func addSshFlags(cmd *cobra.Command) {
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	addContainerFlag(cmd)
	cmd.Flags().Bool("setup", false, "Re-run the SSH access setup (key + Host block) before connecting, even if an alias already exists")
	cmd.Flags().String("setup-external", "", "Run on the Docker host: print a paste-on-the-client snippet (ProxyCommand jump) instead of connecting: USER@HOST")
	cmd.Flags().String("via", "", "Reach the container through an existing SSH connection to its Docker host (requires --container): USER@HOST or an ssh-config alias")
	cmd.Flags().String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	cmd.Flags().String("user", sshdefaults.User, "SSH user inside the container")
	cmd.Flags().Bool("forward", false, "Also open SSH port-forwarding tunnels for the session (prompted when interactive and omitted)")
	cmd.Flags().String("ports", "", "Ports to forward, e.g. '3000,8080:80' (implies --forward; skips the prompt)")

	_ = cmd.RegisterFlagCompletionFunc("via", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("setup-external", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("key")
}

func runSsh(cmd *cobra.Command, args []string) error {
	// --setup-external configures the machine you connect FROM (it prints a
	// snippet); it never dials the target from here, so it short-circuits before
	// any target resolution or connection.
	if external, _ := cmd.Flags().GetString("setup-external"); external != "" {
		_, err := runSshSetup(cmd)
		return err
	}

	interactive := interactiveFlag(cmd)
	forceSetup, _ := cmd.Flags().GetBool("setup")

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	containerExplicit := cmd.Flags().Changed("container")
	via, _ := cmd.Flags().GetString("via")
	if via != "" && !containerExplicit {
		return fmt.Errorf("--via requires --container <name>: there is no local compose project describing a container on a remote host")
	}

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
	// Managed blocks live in the CLI's own SSH config now. Lift any left in the
	// user's ~/.ssh/config by an older version before looking one up, so an
	// already-configured alias is found instead of being set up a second time.
	if moved, merr := ssh.MigrateManagedBlocks(); merr != nil {
		return merr
	} else if len(moved) > 0 {
		console.Ok(fmt.Sprintf("Moved %d managed Host block(s) out of %s", len(moved), domain.UserSSHConfigPath()))
	}
	alias, ok, err := ssh.ManagedAlias(kind, ref)
	if err != nil {
		return err
	}

	if forceSetup || !ok {
		if !ok && !forceSetup {
			console.Info("No SSH access configured yet for '%s'; setting it up...", ref)
		}
		configuredAlias, serr := runSshSetup(cmd)
		if serr != nil {
			return serr
		}
		alias = configuredAlias
	} else if err := refreshExistingTarget(ssh, alias, containerName); err != nil {
		return err
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

// refreshExistingTarget prepares an already configured alias for a reconnect: it
// rules out a stale target deterministically (asking the owning docker daemon by
// name — no SSH), keeps a local block's cached address in step with the
// container's current IP, and re-pins the container's current host keys. A
// container that no longer exists is reported as a stale alias (a hard error, so
// a doomed connection is not attempted); one that is merely stopped is a warning.
//
// The container is anchored by name, so this is where "by name, not by IP" is
// enforced on every connect: a recreated container that came back on a different
// address has its block rewritten (SetManagedHostName) and its old host key
// forgotten before the new one is pinned. A --via/ProxyCommand block has no
// HostName — it already resolves the IP live — so only its host keys are re-pinned,
// under the alias, through the daemon named in its marker.
//
// The host-key steps are best-effort; only a definitively stale target aborts.
func refreshExistingTarget(ssh service.SshService, alias, containerName string) error {
	knownHosts := domain.ManagedKnownHostsPath()
	if updated, err := ssh.EnsureHostKeyPinning(alias, knownHosts); err != nil {
		console.Debug("could not update host-key options for '%s': %v", alias, err)
	} else if updated {
		console.Info("Host '%s' now verifies host keys against %s", alias, knownHosts)
	}

	via, _ := ssh.ManagedViaHost(alias)

	var state service.TargetState
	var ip string
	var liveErr error
	_ = service.WithHostOverride(via, func() error {
		state, ip, liveErr = ssh.ContainerLiveness(containerName)
		return nil
	})
	if liveErr != nil {
		// The daemon could not be queried (typically a --via host that is down):
		// we cannot rule the target stale, so re-pin best-effort and let ssh report
		// the real failure rather than blocking on an unverifiable guess.
		console.Debug("could not verify target '%s': %v", containerName, liveErr)
		pinExistingHostKeys(ssh, alias, containerName, via)
		return nil
	}

	switch state {
	case service.TargetAbsent:
		return fmt.Errorf("container '%s' no longer exists; alias '%s' is stale — run 'devcontainer-cli ssh --setup' to reconfigure it, or 'devcontainer-cli clean ssh' to remove it", containerName, alias)
	case service.TargetStopped:
		console.Warn("Container '%s' is not running; start it before connecting (e.g. 'devcontainer-cli start').", containerName)
		return nil
	}

	refreshLocalHostName(ssh, alias, containerName, ip)
	pinExistingHostKeys(ssh, alias, containerName, via)
	return nil
}

// refreshLocalHostName rewrites a local block's HostName when the container has
// come back on a new IP, so the alias tracks the container by name. It is a no-op
// for a --via/ProxyCommand block (no HostName to update) and when the address is
// already current. The stale address' host key is forgotten so it cannot linger.
func refreshLocalHostName(ssh service.SshService, alias, containerName, ip string) {
	if ip == "" {
		return
	}
	current, err := ssh.AliasHostName(alias)
	if err != nil || current == "" || current == ip {
		return
	}
	changed, err := ssh.SetManagedHostName(alias, ip)
	if err != nil {
		console.Debug("could not refresh HostName for '%s': %v", alias, err)
		return
	}
	if !changed {
		return
	}
	console.Info("Container '%s' is now at %s; updated Host '%s'.", containerName, ip, alias)
	if _, err := ssh.ForgetHostKeys([]string{current}); err != nil {
		console.Debug("could not forget stale host key %s: %v", current, err)
	}
}

// pinExistingHostKeys re-pins the container's current host keys under the identity
// ssh verifies for the block: the HostName (a local block's current IP) read from
// the local daemon, or — for a --via/ProxyCommand block, which has no HostName —
// the alias itself, read through the --via daemon. Pinning a --via block under an
// IP would never be consulted, which is what kept a rebuilt --via container
// aborting with "REMOTE HOST IDENTIFICATION HAS CHANGED".
func pinExistingHostKeys(ssh service.SshService, alias, containerName, via string) {
	host, err := ssh.AliasHostName(alias)
	if err != nil {
		return
	}
	if host != "" {
		if err := ssh.PinContainerHostKeys(containerName, host); err != nil {
			console.Debug("could not pin host key for %s: %v", host, err)
		}
		return
	}
	if via == "" {
		return
	}
	if err := service.WithHostOverride(via, func() error {
		return ssh.PinContainerHostKeys(containerName, alias)
	}); err != nil {
		console.Debug("could not pin host key for %s: %v", alias, err)
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
