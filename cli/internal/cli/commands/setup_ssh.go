package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newSetupSshCommand()) }

type setupSshFlags struct {
	remote              string
	alias               string
	aliasExplicit       bool
	key                 string
	port                string
	mode                string
	assumeYes           bool
	container           string
	containerExplicit   bool
	service             string
	serviceExplicit     bool
	composeFile         string
	composeFileExplicit bool
	user                string
}

func newSetupSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "setup-ssh",
		Short:        "Run automated SSH setup (see: setup-ssh --help)",
		Long:         "devcontainer-cli setup-ssh — automate SSH key + config for devcontainer-ssh",
		SilenceUsage: true,
		RunE:         runSetupSsh,
	}
	f := cmd.Flags()
	f.String("remote", "", "Configure remote-server access (ProxyCommand mode): USER@HOST")
	f.String("alias", sshdefaults.Alias, "SSH alias to register")
	f.String("key", "", "Private key path (default: ~/.ssh/"+sshdefaults.KeyName+")")
	f.String("port", fmt.Sprintf("%d", domain.ResolveSSHHostPort()), "Port for Windows mode")
	f.String("mode", "", "Force mode: local | windows | remote")
	f.String("container", sshdefaults.ServiceName, "Container name (auto-detected from compose if omitted)")
	f.String("service", sshdefaults.ServiceName, "Compose service name (auto-detected if omitted)")
	f.StringP("compose-file", "f", "", "Compose file path (default: .dc_<workspace>/build/docker-compose.yml)")
	f.String("user", sshdefaults.User, "SSH user inside container")
	f.BoolP("yes", "y", false, `Assume "yes" to all prompts`)

	// Dynamic completions
	_ = cmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"local", "windows", "remote"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("container", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listContainers(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("service", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		compFile, _ := cmd.Flags().GetString("compose-file")
		if compFile == "" {
			cwd, err := currentDir()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			compFile = defaultComposeFile(cwd)
		}
		return listComposeServices(compFile), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("remote", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("alias", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return listSshHosts(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagFilename("compose-file", "yml", "yaml")
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
	g.alias, _ = f.GetString("alias")
	g.aliasExplicit = f.Changed("alias")
	g.key, _ = f.GetString("key")
	if g.key == "" {
		g.key = sshdefaults.DefaultKeyPath()
	}
	g.port, _ = f.GetString("port")
	g.mode, _ = f.GetString("mode")
	g.container, _ = f.GetString("container")
	g.containerExplicit = f.Changed("container")
	g.service, _ = f.GetString("service")
	g.serviceExplicit = f.Changed("service")
	g.composeFile, _ = f.GetString("compose-file")
	g.composeFileExplicit = f.Changed("compose-file")
	if g.composeFile == "" {
		g.composeFile = defaultComposeFile(cwd)
	}
	g.user, _ = f.GetString("user")
	g.assumeYes, _ = f.GetBool("yes")

	if g.remote != "" && !f.Changed("mode") {
		g.mode = "remote"
	}
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
	if f.containerExplicit {
		return service.ComposeTarget{Service: f.service, Container: f.container}, nil
	}

	cwd, err := currentDir()
	if err != nil {
		return service.ComposeTarget{}, err
	}
	composePath := filepath.Join(cwd, f.composeFile)
	services := service.ReadComposeServices(composePath)
	if services == nil {
		picked, err := pick.PickManaged("Select devcontainer to set up SSH for:", pick.PickOptions{
			Interactive: !f.assumeYes,
			AssumeYes:   f.assumeYes,
		})
		if err != nil {
			return service.ComposeTarget{}, err
		}
		return service.ComposeTarget{Service: sshdefaults.ServiceName, Container: picked.Name}, nil
	}

	if target, ok := service.PickDevcontainerService(services); ok {
		if f.serviceExplicit {
			target.Service = f.service
		}
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
	if f.mode != "" {
		return sshdefaults.Mode(f.mode)
	}
	if runtime.GOOS == "windows" {
		return sshdefaults.ModeWindows
	}
	return sshdefaults.ModeLocal
}

func checkPrereqs(mode sshdefaults.Mode) error {
	// Remote mode only renders config text to paste on another machine; it runs
	// no ssh/keygen/docker here, so it needs no local tooling.
	if mode == sshdefaults.ModeRemote {
		return nil
	}
	ssh := service.SshService{Report: console}
	for _, tool := range []string{"ssh", "ssh-keygen"} {
		if !ssh.CommandExists(tool) {
			return fmt.Errorf("missing tool: %s", tool)
		}
	}
	if mode == sshdefaults.ModeLocal || mode == sshdefaults.ModeWindows {
		if !ssh.CommandExists("docker") {
			return fmt.Errorf("missing tool: docker")
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

func fetchPassword(ssh service.SshService, f *setupSshFlags) {
	console.Log("Fetching temporary password from logs...")
	if line, ok := ssh.PasswordFromLogs(f.composeFile, f.service, f.user); ok {
		console.Info("   %s", line)
	} else {
		console.Warn("Could not read password from logs (maybe key already installed).")
	}
}

func genKey(ssh service.SshService, keyPath string) error {
	if fileExists(keyPath) && fileExists(keyPath+".pub") {
		console.Ok(fmt.Sprintf("Key already exists: %s", keyPath))
		return nil
	}
	console.Log(fmt.Sprintf("Generating ed25519 key at %s", keyPath))
	if err := ssh.GenerateKey(keyPath); err != nil {
		return err
	}
	console.Ok("Key generated.")
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
	port     string
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
	return installResult{hostname: "localhost", port: f.port}, nil
}

func buildConfigBlock(mode sshdefaults.Mode, f *setupSshFlags, inst installResult) (string, error) {
	return sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:      mode,
		Alias:     f.alias,
		User:      f.user,
		KeyPath:   f.key,
		Hostname:  inst.hostname,
		Port:      inst.port,
		Remote:    f.remote,
		Container: f.container,
	})
}

// updateSshConfig writes the freshly built Host block into ~/.ssh/config,
// appending it or (after confirmation) replacing an existing block for the
// alias. All file parsing/IO lives in the service layer.
func updateSshConfig(ssh service.SshService, f *setupSshFlags, mode sshdefaults.Mode, inst installResult) error {
	newBlock, err := buildConfigBlock(mode, f, inst)
	if err != nil {
		return err
	}
	configPath, current, err := ssh.ReadSSHConfig()
	if err != nil {
		return err
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
	replace := true
	if !f.assumeYes {
		ok, cerr := console.ConfirmDefault(fmt.Sprintf("Replace existing block for Host '%s'?", f.alias), false)
		if cerr != nil {
			return cerr
		}
		replace = ok
	}
	if !replace {
		console.Warn("Skipping ssh config update.")
		return nil
	}
	backupPath, err := ssh.ReplaceHostBlock(configPath, current, f.alias, newBlock)
	if err != nil {
		return err
	}
	console.Ok(fmt.Sprintf("Backup saved: %s", backupPath))
	console.Ok(fmt.Sprintf("Replaced Host '%s' in %s", f.alias, configPath))
	return nil
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
	if f.containerExplicit {
		if ws := workspaceFromContainer(f.container); ws != "" {
			return ws, nil
		}
	}
	cwd, err := currentDir()
	if err != nil {
		return "", err
	}
	if cfg, _ := domain.LoadConfig(cwd); cfg != nil && cfg.Workspace != "" {
		return cfg.Workspace, nil
	}
	if ws := workspaceFromContainer(f.container); ws != "" {
		return ws, nil
	}
	return domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer"), nil
}

// workspaceFromContainer extracts "<workspace>" from a
// "<workspace>-devcontainer-ssh" container name, or "" when it does not match.
func workspaceFromContainer(containerName string) string {
	if containerName == "" {
		return ""
	}
	re := regexp.MustCompile("^(.+)-" + regexp.QuoteMeta(sshdefaults.ServiceName) + "$")
	if m := re.FindStringSubmatch(containerName); m != nil {
		return m[1]
	}
	return ""
}

func applyWorkspaceDefaults(f *setupSshFlags, workspace string) {
	if workspace == "" {
		return
	}
	home, _ := os.UserHomeDir()
	if f.alias == sshdefaults.Alias {
		f.alias = workspace
	}
	if f.key == sshdefaults.DefaultKeyPath() {
		f.key = filepath.Join(home, ".ssh", "id_"+workspace)
	}
	if !f.containerExplicit && f.container == sshdefaults.ServiceName {
		f.container = workspace + "-" + sshdefaults.ServiceName
	}
	if !f.composeFileExplicit {
		f.composeFile = relativeComposeFile(workspace)
	}
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

	if mode != sshdefaults.ModeRemote {
		target, err := resolveTargetService(f)
		if err != nil {
			return err
		}
		f.service = target.Service
		f.container = target.Container
	}

	workspace, err := deriveWorkspace(f)
	if err != nil {
		return err
	}
	applyWorkspaceDefaults(f, workspace)

	if !f.aliasExplicit && !f.assumeYes {
		alias, err := console.AskDefault("SSH connection name (alias):", f.alias, func(v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("name cannot be empty")
			}
			return nil
		})
		if err != nil {
			return err
		}
		f.alias = alias
	}

	// Remote mode assumes the connection to the docker host already exists
	// (devcontainer-cli runs on that host). It neither generates nor installs
	// keys and writes no local config — it only renders the config block to
	// paste on the connecting machine.
	if mode == sshdefaults.ModeRemote {
		return emitRemoteConfig(f)
	}

	console.Log(fmt.Sprintf("Mode: %s   Workspace: %s   Alias: %s   Key: %s", mode, workspace, f.alias, f.key))
	console.Log(fmt.Sprintf("Service: %s   Container: %s   Compose: %s", f.service, f.container, f.composeFile))

	if err := ensureStack(ssh, f); err != nil {
		return err
	}
	fetchPassword(ssh, f)
	if err := genKey(ssh, f.key); err != nil {
		return err
	}
	inst, err := installKey(ssh, f, mode)
	if err != nil {
		return err
	}
	if err := updateSshConfig(ssh, f, mode, inst); err != nil {
		return err
	}
	testConnection(ssh, f.alias)
	console.Ok(fmt.Sprintf("Done. Connect with:  ssh %s", f.alias))
	return nil
}

// emitRemoteConfig prints the manual step-by-step for remote access — generate a
// key, install its public half into the container through the docker host, then
// add the ProxyCommand block — without running anything locally. In remote mode
// devcontainer-cli runs on the docker host, so these steps belong on the machine
// the user connects FROM.
func emitRemoteConfig(f *setupSshFlags) error {
	block, err := buildConfigBlock(sshdefaults.ModeRemote, f, installResult{})
	if err != nil {
		return err
	}
	displayKey := "~/.ssh/" + filepath.Base(f.key)

	console.NewLine()
	console.Log("Remote setup — run these steps on the machine you'll connect FROM:")
	console.NewLine()

	console.Info("1) Generate an SSH key pair:")
	console.Print("   " + sshdefaults.RemoteKeygenCommand(displayKey) + "\n")
	console.NewLine()

	console.Info("2) Install the public key into the container (through the docker host):")
	console.Print("   " + sshdefaults.RemoteInstallKeyCommand(displayKey, f.remote, f.user, f.container) + "\n")
	console.NewLine()

	console.Info("3) Add this block to that machine's ~/.ssh/config:")
	console.Print(block + "\n")
	console.NewLine()

	console.Success("Then connect with:  ssh %s", f.alias)
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
