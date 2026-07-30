package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// setupSshFlags is the resolved input to the SSH setup flow, shared by the
// 'ssh --setup' and 'ssh --setup-external' entry points and by the automatic
// bootstrap 'ssh' runs when no managed alias exists yet. It is populated from the
// 'ssh' command's flags by collectSetupSshFlags.
type setupSshFlags struct {
	remote            string
	via               string
	alias             string
	key               string
	knownHosts        string
	assumeYes         bool
	interactive       bool
	container         string
	containerExplicit bool
	service           string
	composeFile       string
	user              string
}

func collectSetupSshFlags(cmd *cobra.Command) (*setupSshFlags, error) {
	f := cmd.Flags()
	cwd, err := currentDir()
	if err != nil {
		return nil, err
	}
	interactive := interactiveFlag(cmd)
	g := &setupSshFlags{}
	g.remote, _ = f.GetString("setup-external")
	g.via, _ = f.GetString("via")
	g.alias = sshdefaults.Alias
	g.key, _ = f.GetString("key")
	g.key = domain.ResolveSSHKeyPath(g.key)
	g.knownHosts = domain.ManagedKnownHostsPath()
	g.container, _ = f.GetString("container")
	g.containerExplicit = f.Changed("container")
	g.service = sshdefaults.ServiceName
	g.composeFile = defaultComposeFile(cwd)
	g.user, _ = f.GetString("user")
	g.interactive = interactive
	g.assumeYes = yesFlag(cmd) || !interactive

	return g, nil
}

// resolveTargetService determines the compose service/container to target. The
// two paths are kept separate:
//   - container-driven: an explicit --container fully determines the target (the
//     service name defaults to sshdefaults.ServiceName unless --service is given),
//     so no compose file is read.
//   - workspace-driven: the target is discovered from the project compose file,
//     falling back to the managed-container picker when none exists.
func resolveTargetService(f *setupSshFlags) (service.ComposeTarget, error) {
	cwd, err := currentDir()
	if err != nil {
		return service.ComposeTarget{}, err
	}
	composePath := filepath.Join(cwd, f.composeFile)
	services := service.ReadComposeServices(composePath)
	if services == nil {
		picked, err := pickManagedContainer("Select devcontainer to set up SSH for:", pickContainerOptions{
			Interactive: !f.assumeYes,
			AssumeYes:   f.assumeYes,
		})
		if err != nil {
			return service.ComposeTarget{}, err
		}
		return service.ComposeTarget{Service: sshdefaults.ServiceName, Container: picked.Name}, nil
	}

	if target, ok := service.PickDevcontainerService(services); ok {
		return target, nil
	}

	return selectComposeService(f, services, composePath)
}

// selectComposeService asks the user to choose among the compose services when no
// devcontainer service could be auto-detected.
func selectComposeService(f *setupSshFlags, services map[string]service.ComposeService, composePath string) (service.ComposeTarget, error) {
	keys := make([]string, 0, len(services))
	for key := range services {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return service.ComposeTarget{}, fmt.Errorf("no services found in %s", composePath)
	}
	if f.assumeYes {
		first := keys[0]
		return service.ComposeTarget{Service: first, Container: service.ContainerOf(services[first], first)}, nil
	}
	choices := make([]service.Option, len(keys))
	for i, key := range keys {
		label := key
		if services[key].ContainerName != "" {
			label = fmt.Sprintf("%s (container: %s)", key, services[key].ContainerName)
		}
		choices[i] = service.Option{Value: key, Label: label}
	}
	chosen, err := console.Select("Select SSH service:", choices, service.Option{Value: keys[0]})
	if err != nil {
		return service.ComposeTarget{}, err
	}
	return service.ComposeTarget{Service: chosen.Value, Container: service.ContainerOf(services[chosen.Value], chosen.Value)}, nil
}

func detectMode(f *setupSshFlags) sshdefaults.Mode {
	if f.remote != "" {
		return sshdefaults.ModeRemote
	}

	return sshdefaults.ModeLocal
}

func checkPrereqs(mode sshdefaults.Mode) error {
	ssh := service.SshService{Report: console}
	for _, tool := range []string{"ssh", "ssh-keygen", "docker"} {
		if !ssh.CommandExists(tool) {
			return fmt.Errorf("missing tool: %s", tool)
		}
	}
	return nil
}

const containerStartTimeoutSeconds = 20

// ensureStack makes sure the target container is running, starting the compose
// stack (with confirmation) and waiting for it when necessary.
func ensureStack(ssh service.SshService, f *setupSshFlags) error {
	if ssh.ContainerRunning(f.container) {
		console.Ok(fmt.Sprintf("Container '%s' is running.", f.container))
		return nil
	}

	console.Warn("Container '%s' not running.", f.container)

	if f.containerExplicit || f.composeFile == "" {
		return fmt.Errorf("container '%s' is not running. Please start it manually first", f.container)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	composePath := filepath.Join(cwd, f.composeFile)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("compose file not found: %s\nRun 'devcontainer-cli' first to generate it, or pass -f <path> to specify a different compose file", composePath)
	}

	proceed := true
	if !f.assumeYes {
		ok, err := console.ConfirmDefault("Start it now with docker compose up -d?", true)
		if err != nil {
			return err
		}
		proceed = ok
	}

	if !proceed {
		return fmt.Errorf("aborting — stack must be running")
	}

	if err := ssh.ComposeUp(f.composeFile); err != nil {
		return err
	}

	for remaining := containerStartTimeoutSeconds; remaining > 0 && !ssh.ContainerRunning(f.container); remaining-- {
		time.Sleep(time.Second)
	}

	if !ssh.ContainerRunning(f.container) {
		return fmt.Errorf("container failed to start")
	}

	return nil
}

func genKey(ssh service.SshService, keyPath string) error {
	created, err := ssh.EnsureKey(keyPath)
	if err != nil {
		return err
	}

	if created {
		console.Ok(fmt.Sprintf("Generated ed25519 key at %s", keyPath))
	} else {
		console.Ok(fmt.Sprintf("Key already exists: %s", keyPath))
	}

	return nil
}

// containerIP resolves a single usable IP for the container, prompting to choose
// when the container is attached to more than one network.
func containerIP(ssh service.SshService, container string, f *setupSshFlags) (string, error) {
	entries, err := ssh.ContainerIPs(container)
	if err != nil {
		return "", err
	}
	switch len(entries) {
	case 0:
		return "", nil
	case 1:
		return entries[0].IP, nil
	}
	if f.assumeYes {
		console.Warn("Container '%s' is on %d networks; using '%s' (%s).", container, len(entries), entries[0].Network, entries[0].IP)
		return entries[0].IP, nil
	}
	choices := make([]service.Option, len(entries))
	for i, entry := range entries {
		choices[i] = service.Option{Value: entry.IP, Label: fmt.Sprintf("%s (%s)", entry.Network, entry.IP)}
	}
	sel, err := console.Select("Container is on multiple networks. Select one:", choices, choices[0])
	return sel.Value, err
}

type installResult struct {
	hostname string
}

func installKey(ssh service.SshService, f *setupSshFlags, mode sshdefaults.Mode) (installResult, error) {
	pub, err := os.ReadFile(f.key + ".pub")
	if err != nil {
		return installResult{}, err
	}
	script := sshdefaults.AuthorizedKeysInstallScript()

	// --via keeps mode == ModeLocal (see detectMode) but its docker calls are
	// redirected at the remote daemon (see performSetupSsh), so — unlike
	// legacy --remote — it can resolve the container's IP to pin a host key
	// for this connection. buildConfigBlock re-resolves the IP live on every
	// connect rather than using this one.
	if mode == sshdefaults.ModeLocal || f.via != "" {
		ip, ierr := containerIP(ssh, f.container, f)
		if ierr != nil {
			return installResult{}, ierr
		}
		if ip == "" {
			return installResult{}, fmt.Errorf("could not resolve container IP")
		}
		console.Log(fmt.Sprintf("Installing public key into %s (%s) via docker exec...", f.container, ip))
		if err := ssh.InstallKeyLocal(service.InstallKeySpec{
			PublicKey: pub,
			User:      f.user,
			Container: f.container,
			Script:    script,
		}); err != nil {
			return installResult{}, err
		}
		return installResult{hostname: ip}, nil
	}

	console.Log(fmt.Sprintf("Installing public key into %s via docker exec...", f.container))
	if err := ssh.InstallKeyLocal(service.InstallKeySpec{
		PublicKey: pub,
		User:      f.user,
		Container: f.container,
		Script:    script,
	}); err != nil {
		return installResult{}, err
	}
	return installResult{}, nil
}

// buildConfigBlock renders the Host stanza for f. --via renders as
// sshdefaults.ModeRemote (Remote: f.via) even though the caller's mode stays
// ModeLocal for orchestration purposes (see performSetupSsh/installKey) — only
// the rendering shape changes.
func buildConfigBlock(mode sshdefaults.Mode, f *setupSshFlags, inst installResult, kind sshdefaults.Kind, ref string) (string, error) {
	renderMode := mode
	remote := f.remote
	if f.via != "" {
		renderMode = sshdefaults.ModeRemote
		remote = f.via
	}
	return sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:           renderMode,
		Alias:          f.alias,
		User:           f.user,
		KeyPath:        f.key,
		Hostname:       inst.hostname,
		Remote:         remote,
		Container:      f.container,
		KnownHostsFile: f.knownHosts,
		Kind:           kind,
		Ref:            ref,
	})
}

// pinHostKeys records the container's current ssh host keys for the identity the
// Host block verifies against, so the very first connection — and every one after
// an image rebuild regenerated those keys — verifies instead of prompting or
// failing with "REMOTE HOST IDENTIFICATION HAS CHANGED". It is best-effort: when
// the keys cannot be read the block still works, ssh just falls back to
// accept-new.
//
// A local block has a HostName (the container IP) and ssh checks the key under
// that address. A --via block renders as ModeRemote — a ProxyCommand with no
// HostName — so ssh checks (and we must pin) the key under the alias itself;
// pinning under the IP there would never be consulted.
func pinHostKeys(ssh service.SshService, f *setupSshFlags, inst installResult) {
	host := inst.hostname
	if f.via != "" {
		host = f.alias
	}
	if host == "" {
		return
	}
	if err := ssh.PinContainerHostKeys(f.container, host); err != nil {
		console.Warn("Could not pin the container host key (%v); ssh will trust it on first use.", err)
		return
	}
	console.Ok(fmt.Sprintf("Pinned host key of %s in %s", host, f.knownHosts))
}

// aliasAction is the user's decision when the target Host alias already exists
// in either SSH config.
type aliasAction int

const (
	aliasOverwrite aliasAction = iota // replace the existing block in place
	aliasRename                       // write the block under a different alias
	aliasSkip                         // leave the config untouched
)

// aliasPrompter is the subset of the console used to resolve an existing-alias
// conflict; carving it out lets tests script the overwrite/rename/skip decision
// without a real terminal.
type aliasPrompter interface {
	Select(prompt string, choices []service.Option, initial service.Option) (service.Option, error)
	AskDefault(prompt string, initial string, validate func(string) error) (string, error)
}

// resolveAliasConflict asks the user how to handle an already-defined Host alias:
// overwrite it, write the block under a new alias, or skip the config update.
// On aliasRename it updates f.alias to the freshly chosen name. In assume-yes
// mode it overwrites without prompting, preserving the non-interactive default.
func resolveAliasConflict(p aliasPrompter, f *setupSshFlags) (aliasAction, error) {
	if f.assumeYes {
		return aliasOverwrite, nil
	}
	choice, err := p.Select(
		fmt.Sprintf("Host '%s' already exists. What do you want to do?", f.alias),
		[]service.Option{
			{Value: "overwrite", Label: "Overwrite the existing alias"},
			{Value: "rename", Label: "Choose a new alias"},
			{Value: "skip", Label: "Skip ssh config update"},
		},
		service.Option{Value: "overwrite", Label: "Overwrite the existing alias"},
	)
	if err != nil {
		return aliasSkip, err
	}
	switch choice.Value {
	case "rename":
		alias, aerr := p.AskDefault("New SSH connection name (alias):", f.alias, validateAliasName)
		if aerr != nil {
			return aliasSkip, aerr
		}
		f.alias = strings.TrimSpace(alias)
		return aliasRename, nil
	case "skip":
		return aliasSkip, nil
	default:
		return aliasOverwrite, nil
	}
}

// updateSshConfig writes the freshly built Host block into the CLI-managed SSH
// config file, appending it when the alias is free or, when it collides, letting
// the user overwrite the existing block or pick a new alias (re-checking the new
// name for a fresh conflict).
//
// The user's own ~/.ssh/config is never rewritten here: it only gains the
// Include directive (EnsureInclude) that pulls the managed file in, and any
// managed block older versions left inside it is lifted out first
// (MigrateManagedBlocks). Conflicts are still checked against BOTH files, so
// the CLI cannot silently shadow a Host the user wrote by hand. All file
// parsing/IO lives in the service layer.
func updateSshConfig(ssh service.SshService, f *setupSshFlags, mode sshdefaults.Mode, inst installResult, kind sshdefaults.Kind, ref string) error {
	moved, err := ssh.MigrateManagedBlocks()
	if err != nil {
		return err
	}
	for _, m := range moved {
		console.Ok(fmt.Sprintf("Moved managed Host '%s' out of %s", m.Alias, domain.UserSSHConfigPath()))
	}

	added, err := ssh.EnsureInclude()
	if err != nil {
		return err
	}

	configPath, current, err := ssh.ReadManagedConfig()
	if err != nil {
		return err
	}
	if added {
		console.Ok(fmt.Sprintf("Added 'Include' for %s to %s", configPath, domain.UserSSHConfigPath()))
	}

	for {
		newBlock, berr := buildConfigBlock(mode, f, inst, kind, ref)
		if berr != nil {
			return berr
		}

		conflict, cerr := ssh.FindHostAliasConflict(f.alias)
		if cerr != nil {
			return cerr
		}
		if conflict == nil {
			if err := ssh.AppendHostBlock(configPath, current, newBlock); err != nil {
				return err
			}
			console.Ok(fmt.Sprintf("Appended Host '%s' to %s", f.alias, configPath))
			return nil
		}

		console.Warn("Host '%s' already defined in %s", f.alias, conflict.Path)
		if !conflict.Managed {
			console.Warn("That is your own config, which this CLI never rewrites. Overwriting writes the")
			console.Warn("block to %s instead, which wins because the Include sits above your blocks.", configPath)
		}
		console.Info("---- existing ----")
		console.Print(conflict.Block + "\n")
		console.Info("---- proposed ----")
		console.Print(newBlock + "\n")

		action, aerr := resolveAliasConflict(console, f)
		if aerr != nil {
			return aerr
		}
		switch action {
		case aliasRename:
			// Re-loop: rebuild the block for the new alias and re-check for a
			// collision against the (still unmodified) configs.
			continue
		case aliasSkip:
			console.Warn("Skipping ssh config update.")
			return nil
		default: // aliasOverwrite
			backupPath, rerr := ssh.ReplaceHostBlock(configPath, current, f.alias, newBlock)
			if rerr != nil {
				return rerr
			}
			console.Ok(fmt.Sprintf("Backup saved: %s", backupPath))
			console.Ok(fmt.Sprintf("Replaced Host '%s' in %s", f.alias, configPath))
			return nil
		}
	}
}

// verifyTarget checks a freshly configured target in two tiers. First it rules
// out a stale target deterministically: it asks the owning docker daemon (local,
// or the --via daemon when this runs under WithHostOverride) whether the
// container is up, by name — no SSH, no side effects, so it always runs. Only if
// that passes does it offer the intrusive tier: a real SSH probe that opens an
// actual connection, which is shown and confirmed first so it is never run
// silently.
func verifyTarget(ssh service.SshService, f *setupSshFlags) {
	if !ssh.ContainerRunning(f.container) {
		console.Warn("Container '%s' is not running; 'ssh %s' will fail until it is started.", f.container, f.alias)
		return
	}
	if !confirmLiveProbe(f, f.alias) {
		return
	}
	testConnection(ssh, f.alias)
}

// confirmLiveProbe decides whether to open the real SSH probe. It never opens one
// non-interactively (nothing asked for it), auto-confirms under -y, and otherwise
// asks — so a simulated connection is always either explicitly requested or
// explicitly confirmed.
func confirmLiveProbe(f *setupSshFlags, alias string) bool {
	if !f.interactive {
		return false
	}
	if f.assumeYes {
		return true
	}
	proceed, err := console.ConfirmDefault(fmt.Sprintf("Verify now by opening a real SSH connection to '%s'?", alias), true)
	return err == nil && proceed
}

func testConnection(ssh service.SshService, alias string) {
	console.Log(fmt.Sprintf("Testing ssh %s ...", alias))
	switch ssh.TestConnection(alias) {
	case service.SSHTestOK:
		console.Ok(fmt.Sprintf("SSH alias '%s' works.", alias))
	case service.SSHTestTimeout:
		console.Warn("SSH test timed out after 15s. Try manually:  ssh %s", alias)
	default:
		console.Warn("SSH test inconclusive. Try manually:  ssh %s", alias)
	}
}

// deriveWorkspace resolves the workspace backing the alias/key/compose defaults,
// keeping the two paths separate:
//   - container-driven: when --container is explicit, the workspace is parsed from
//     that container name so the defaults match the targeted container, not the
//     current directory.
//   - workspace-driven: otherwise it comes from the project config, then the
//     container name, then the sanitized directory name.
func deriveWorkspace(f *setupSshFlags) (string, error) {
	cwd, err := currentDir()
	if err != nil {
		return "", err
	}
	if cfg, _ := domain.LoadConfig(cwd); cfg != nil && cfg.Workspace != "" {
		return cfg.Workspace, nil
	}
	return domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer"), nil
}

func applyWorkspaceDefaults(f *setupSshFlags, workspace string) {
	if workspace == "" {
		return
	}
	if f.alias == sshdefaults.Alias {
		f.alias = workspace
	}
	f.container = workspace + "-" + sshdefaults.ServiceName
	f.composeFile = relativeComposeFile(workspace)
}

// runSshSetup drives the SSH setup flow for the 'ssh' command: it resolves the
// target (a loose --container or the project's workspace devcontainer), writes
// the managed Host block (or, in --setup-external mode, prints the self-contained
// snippet for the machine you connect FROM), and returns the configured alias so
// the caller can connect through it. In --setup-external mode the returned alias
// is not connectable from here (it targets another machine); the caller must not
// dial it.
func runSshSetup(cmd *cobra.Command) (alias string, err error) {
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		return "", err
	}
	if f.remote != "" && f.via != "" {
		return "", fmt.Errorf("--setup-external and --via are mutually exclusive: use one or the other")
	}
	if f.via != "" && !f.containerExplicit {
		return "", fmt.Errorf("--via requires --container <name>: there is no local compose project describing a container on a remote host")
	}

	ssh := service.SshService{Report: console}
	mode := detectMode(f)
	if err := checkPrereqs(mode); err != nil {
		return "", err
	}

	var workspace string
	if f.containerExplicit {
		// Loose container mode: bypass workspace logic completely.
		f.composeFile = ""
		f.service = sshdefaults.ServiceName
		if f.alias == sshdefaults.Alias {
			f.alias = f.container
		}
		workspace = sshNoWorkspace
	} else {
		// Workspace mode: discover target container and workspace context.
		target, err := resolveTargetService(f)
		if err != nil {
			return "", err
		}
		f.service = target.Service
		f.container = target.Container

		workspace, err = deriveWorkspace(f)
		if err != nil {
			return "", err
		}
		applyWorkspaceDefaults(f, workspace)
	}

	if f.remote == "" && !f.assumeYes {
		chosen, err := console.AskDefault("SSH connection name (alias):", f.alias, validateAliasName)
		if err != nil {
			return "", err
		}
		f.alias = chosen
	}

	if err := performSetupSsh(ssh, f, mode, workspace); err != nil {
		return "", err
	}
	return f.alias, nil
}

// performSetupSsh runs the core of the setup-ssh flow once mode, workspace and
// alias are resolved: ensure the stack is running, generate/install the shared
// key, write (or print, in remote mode) the ~/.ssh/config Host block, and test
// the connection. Shared between the explicit 'setup-ssh' command and the
// automatic bootstrap 'ssh' runs when no managed alias exists yet for the
// workspace.
func performSetupSsh(ssh service.SshService, f *setupSshFlags, mode sshdefaults.Mode, workspace string) error {
	console.Log(fmt.Sprintf("Mode: %s   Workspace: %s   Alias: %s   Key: %s", mode, workspace, f.alias, f.key))
	console.Log(fmt.Sprintf("Service: %s   Container: %s   Compose: %s", f.service, f.container, f.composeFile))

	// Redirects ensureStack's running check, key install, and host-key read at
	// the daemon behind --via; a no-op when f.via is empty.
	return service.WithHostOverride(f.via, func() error {
		if err := ensureStack(ssh, f); err != nil {
			return err
		}

		if err := genKey(ssh, f.key); err != nil {
			return err
		}

		inst, err := installKey(ssh, f, mode)
		if err != nil {
			return err
		}

		// The marker tags the generated block so destroy/clean-ssh can remove it. Loose
		// container mode keys the block by container name; workspace mode by the (unique)
		// workspace name.
		markerKind := sshdefaults.KindWorkspace
		markerRef := workspace
		if f.containerExplicit {
			markerKind = sshdefaults.KindContainer
			markerRef = f.container
		}

		if mode == sshdefaults.ModeRemote {
			return emitRemoteConfig(ssh, f, markerKind, markerRef)
		}

		if err := updateSshConfig(ssh, f, mode, inst, markerKind, markerRef); err != nil {
			return err
		}

		// After updateSshConfig so an interactively-renamed alias is the one pinned:
		// a --via block pins under the alias, and resolveAliasConflict may have
		// changed it.
		pinHostKeys(ssh, f, inst)

		verifyTarget(ssh, f)
		console.Ok(fmt.Sprintf("Done. Connect with:  ssh %s", f.alias))
		return nil
	})
}

// emitRemoteConfig hands over the shared managed key to the machine the user
// connects FROM. In remote mode devcontainer-cli runs on the docker host, so it
// ensures the managed key exists here, reads its private+public bytes, and
// prints one self-contained snippet (RemoteExportScript) that writes that same
// key on the connecting machine and appends the ProxyCommand block to its
// ~/.ssh/config. The snippet contains a PRIVATE key. The shared public key is
// injected into the target container prior to invoking this function, so the
// connecting machine only needs the key + config.
func emitRemoteConfig(ssh service.SshService, f *setupSshFlags, kind sshdefaults.Kind, ref string) error {
	displayKey := "~/.ssh/" + sshdefaults.KeyName

	// The block is pasted into the connecting machine's ~/.ssh/config, whose home
	// is not this host's, so IdentityFile and UserKnownHostsFile must use the ~
	// form, not f.key's / f.knownHosts' absolute local paths.
	rf := *f
	rf.key = displayKey
	rf.knownHosts = sshdefaults.RemoteKnownHostsPath
	block, err := buildConfigBlock(sshdefaults.ModeRemote, &rf, installResult{}, kind, ref)
	if err != nil {
		return err
	}

	if _, err := ssh.EnsureKey(f.key); err != nil {
		return err
	}

	priv, err := ssh.PrivateKey(f.key)
	if err != nil {
		return err
	}

	pub, err := ssh.PublicKey(f.key)
	if err != nil {
		return err
	}

	exportScript := sshdefaults.RemoteExportScript(sshdefaults.RemoteExportOptions{
		PrivateKey:  priv,
		PublicKey:   pub,
		ConfigBlock: block,
	})

	console.NewLine()
	console.Log("Remote setup — run these steps on the machine you'll connect FROM:")
	console.NewLine()
	console.Warn("The snippet below contains a PRIVATE key — treat it as a secret.")
	console.NewLine()

	console.Info("Install the shared key + ssh config (paste on the connecting machine):")
	console.Print(exportScript + "\n")
	console.NewLine()

	console.Success("Then connect with:  ssh %s", f.alias)
	return nil
}

// validateAliasName rejects a blank SSH alias; it is shared by the upfront alias
// prompt and the rename branch of resolveAliasConflict.
func validateAliasName(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
