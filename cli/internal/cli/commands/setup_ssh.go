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

func init() { register(newSetupSshCommand()) }

type setupSshFlags struct {
	remote            string
	alias             string
	key               string
	knownHosts        string
	assumeYes         bool
	container         string
	containerExplicit bool
	service           string
	composeFile       string
	user              string
}

func newSetupSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup-ssh",
		Short: "Set up SSH key + config so you can 'ssh' into the devcontainer",
		Long: `devcontainer-cli setup-ssh — automate end-to-end SSH access to a devcontainer.

It generates (once) a single shared ed25519 key managed by the CLI, makes sure
the target container is running (offering to start the stack if not), installs
the public key into the container's authorized_keys, resolves the container's IP,
and appends a ready-to-use Host block to your ~/.ssh/config — then tests the
connection. Afterwards you connect with a plain 'ssh <alias>'. If the alias
already exists you're asked to overwrite it, pick a new name, or skip.

Host keys are pinned in a known_hosts file owned by the CLI, not in your global
~/.ssh/known_hosts, and are read from the container through docker rather than
trusted on first sight. A rebuilt image therefore never greets you with "REMOTE
HOST IDENTIFICATION HAS CHANGED" for what is really a fresh container.

Two modes:
  local (default)  Configure direct SSH from this machine into a local container.
  remote (--remote USER@HOST)  This CLI runs on the Docker host; it prints a
                   self-contained snippet (containing the PRIVATE key) to paste
                   on the machine you connect FROM, setting up a ProxyCommand jump.`,
		Example: `  # Set up SSH for the project's devcontainer, then connect
  devcontainer-cli setup-ssh
  ssh <workspace>

  # Target a specific container unattended
  devcontainer-cli setup-ssh --container dc-ssh --yes

  # Remote/jump-host setup (run on the Docker host)
  devcontainer-cli setup-ssh --remote me@docker-host`,
		SilenceUsage: true,
		RunE:         runSetupSsh,
	}
	f := cmd.Flags()
	f.String("remote", "", "Configure remote-server access (ProxyCommand mode): USER@HOST")
	f.String("key", "", "Private key path (default: the shared managed key under the CLI config dir)")
	f.StringP("container", "c", sshdefaults.ServiceName, "Container name (auto-detected from compose if omitted)")
	f.String("user", sshdefaults.User, "SSH user inside container")
	f.BoolP("yes", "y", false, `Assume "yes" to all prompts (overwrite a conflicting alias, auto-start the stack)`)

	_ = cmd.RegisterFlagCompletionFunc("container", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listContainers(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("remote", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("key")

	return cmd
}

func collectSetupSshFlags(cmd *cobra.Command) (*setupSshFlags, error) {
	f := cmd.Flags()
	cwd, err := currentDir()
	if err != nil {
		return nil, err
	}
	g := &setupSshFlags{}
	g.remote, _ = f.GetString("remote")
	g.alias = sshdefaults.Alias
	g.key, _ = f.GetString("key")
	g.key = domain.ResolveSSHKeyPath(g.key)
	g.knownHosts = domain.ManagedKnownHostsPath()
	g.container, _ = f.GetString("container")
	g.containerExplicit = f.Changed("container")
	g.service = sshdefaults.ServiceName
	g.composeFile = defaultComposeFile(cwd)
	g.user, _ = f.GetString("user")
	g.assumeYes, _ = f.GetBool("yes")

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

	if mode == sshdefaults.ModeLocal {
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

func buildConfigBlock(mode sshdefaults.Mode, f *setupSshFlags, inst installResult, kind sshdefaults.Kind, ref string) (string, error) {
	return sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:           mode,
		Alias:          f.alias,
		User:           f.user,
		KeyPath:        f.key,
		Hostname:       inst.hostname,
		Remote:         f.remote,
		Container:      f.container,
		KnownHostsFile: f.knownHosts,
		Kind:           kind,
		Ref:            ref,
	})
}

// pinHostKeys records the container's current ssh host keys for the address the
// Host block dials, so the very first connection — and every one after an image
// rebuild regenerated those keys — verifies instead of prompting or failing with
// "REMOTE HOST IDENTIFICATION HAS CHANGED". It is best-effort: when the keys
// cannot be read the block still works, ssh just falls back to accept-new.
func pinHostKeys(ssh service.SshService, f *setupSshFlags, inst installResult) {
	if inst.hostname == "" {
		return
	}
	if err := ssh.PinContainerHostKeys(f.container, inst.hostname); err != nil {
		console.Warn("Could not pin the container host key (%v); ssh will trust it on first use.", err)
		return
	}
	console.Ok(fmt.Sprintf("Pinned host key of %s in %s", inst.hostname, f.knownHosts))
}

// aliasAction is the user's decision when the target Host alias already exists
// in ~/.ssh/config.
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

// updateSshConfig writes the freshly built Host block into ~/.ssh/config,
// appending it when the alias is free or, when it collides, letting the user
// overwrite the existing block or pick a new alias (re-checking the new name for
// a fresh conflict). All file parsing/IO lives in the service layer.
func updateSshConfig(ssh service.SshService, f *setupSshFlags, mode sshdefaults.Mode, inst installResult, kind sshdefaults.Kind, ref string) error {
	configPath, current, err := ssh.ReadSSHConfig()
	if err != nil {
		return err
	}

	for {
		newBlock, berr := buildConfigBlock(mode, f, inst, kind, ref)
		if berr != nil {
			return berr
		}

		if !service.HasHostAlias(current, f.alias) {
			if err := ssh.AppendHostBlock(configPath, current, newBlock); err != nil {
				return err
			}
			console.Ok(fmt.Sprintf("Appended Host '%s' to %s", f.alias, configPath))
			return nil
		}

		console.Warn("Host '%s' already defined in %s", f.alias, configPath)
		console.Info("---- existing ----")
		console.Print(service.ExtractHostBlock(current, f.alias) + "\n")
		console.Info("---- proposed ----")
		console.Print(newBlock + "\n")

		action, aerr := resolveAliasConflict(console, f)
		if aerr != nil {
			return aerr
		}
		switch action {
		case aliasRename:
			// Re-loop: rebuild the block for the new alias and re-check for a
			// collision against the (still unmodified) config.
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

func runSetupSsh(cmd *cobra.Command, _ []string) error {
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		return err
	}

	ssh := service.SshService{Report: console}
	mode := detectMode(f)
	if err := checkPrereqs(mode); err != nil {
		return err
	}

	var workspace string
	if f.containerExplicit {
		// Loose container mode: bypass workspace logic completely.
		f.composeFile = ""
		f.service = sshdefaults.ServiceName
		if f.alias == sshdefaults.Alias {
			f.alias = f.container
		}
		workspace = "(none)"
	} else {
		// Workspace mode: discover target container and workspace context.
		target, err := resolveTargetService(f)
		if err != nil {
			return err
		}
		f.service = target.Service
		f.container = target.Container

		workspace, err = deriveWorkspace(f)
		if err != nil {
			return err
		}
		applyWorkspaceDefaults(f, workspace)
	}

	if f.remote == "" && !f.assumeYes {
		alias, err := console.AskDefault("SSH connection name (alias):", f.alias, validateAliasName)
		if err != nil {
			return err
		}
		f.alias = alias
	}

	return performSetupSsh(ssh, f, mode, workspace)
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

	pinHostKeys(ssh, f, inst)

	if err := updateSshConfig(ssh, f, mode, inst, markerKind, markerRef); err != nil {
		return err
	}

	testConnection(ssh, f.alias)
	console.Ok(fmt.Sprintf("Done. Connect with:  ssh %s", f.alias))
	return nil
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
