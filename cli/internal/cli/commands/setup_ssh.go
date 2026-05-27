package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
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
	f.String("port", fmt.Sprintf("%d", sshdefaults.WindowsPort), "Port for Windows mode")
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
			cwd, _ := os.Getwd()
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

func defaultComposeFile(cwd string) string {
	cfg, _ := domain.LoadConfig(cwd)
	ws := ""
	if cfg != nil {
		ws = cfg.Workspace
	}
	if ws == "" {
		ws = domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer")
	}
	return fmt.Sprintf(".dc_%s/build/docker-compose.yml", ws)
}

func collectSetupSshFlags(cmd *cobra.Command) *setupSshFlags {
	f := cmd.Flags()
	cwd, _ := os.Getwd()
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
	return g
}

func which(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

type composeServiceInfo struct {
	service   string
	container string
}

type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	ContainerName string `yaml:"container_name"`
}

func readComposeServices(composeFile string) map[string]composeService {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil
	}
	var doc composeDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if len(doc.Services) == 0 {
		return nil
	}
	return doc.Services
}

func containerOf(svc composeService, key string) string {
	if svc.ContainerName != "" {
		return svc.ContainerName
	}
	return key
}

func pickDevcontainerService(services map[string]composeService) *composeServiceInfo {
	for key, svc := range services {
		if key == sshdefaults.ServiceName || containerOf(svc, key) == sshdefaults.ServiceName {
			return &composeServiceInfo{service: key, container: containerOf(svc, key)}
		}
	}
	var candidates []composeServiceInfo
	for key, svc := range services {
		c := containerOf(svc, key)
		if strings.Contains(key, "devcontainer") || strings.Contains(c, "devcontainer") {
			candidates = append(candidates, composeServiceInfo{service: key, container: c})
		}
	}
	if len(candidates) == 1 {
		return &candidates[0]
	}
	return nil
}

func resolveTargetService(f *setupSshFlags) (composeServiceInfo, error) {
	if f.containerExplicit && f.serviceExplicit {
		return composeServiceInfo{service: f.service, container: f.container}, nil
	}
	cwd, _ := os.Getwd()
	composePath := filepath.Join(cwd, f.composeFile)
	services := readComposeServices(composePath)
	if services == nil {
		if f.containerExplicit {
			return composeServiceInfo{service: f.service, container: f.container}, nil
		}
		picked, err := pick.PickManaged("Select devcontainer to set up SSH for:", pick.PickOptions{
			Interactive: !f.assumeYes,
			AssumeYes:   f.assumeYes,
		})
		if err != nil {
			return composeServiceInfo{}, err
		}
		return composeServiceInfo{service: sshdefaults.ServiceName, container: picked.Name}, nil
	}

	if picked := pickDevcontainerService(services); picked != nil {
		if f.containerExplicit {
			picked.container = f.container
		}
		if f.serviceExplicit {
			picked.service = f.service
		}
		return *picked, nil
	}

	var keys []string
	for key := range services {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return composeServiceInfo{}, fmt.Errorf("no services found in %s", composePath)
	}
	if f.assumeYes {
		first := keys[0]
		return composeServiceInfo{service: first, container: containerOf(services[first], first)}, nil
	}
	choices := make([]prompt.Choice, len(keys))
	for i, key := range keys {
		label := key
		if services[key].ContainerName != "" {
			label = fmt.Sprintf("%s (container: %s)", key, services[key].ContainerName)
		}
		choices[i] = prompt.Choice{Value: key, Label: label}
	}
	chosen, err := prompt.Select("Select SSH service:", choices, keys[0])
	if err != nil {
		return composeServiceInfo{}, err
	}
	return composeServiceInfo{service: chosen, container: containerOf(services[chosen], chosen)}, nil
}

func detectMode(f *setupSshFlags) string {
	if f.mode != "" {
		return f.mode
	}
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "local"
}

func checkPrereqs(mode string) error {
	for _, t := range []string{"ssh", "ssh-keygen"} {
		if !which(t) {
			return fmt.Errorf("missing tool: %s", t)
		}
	}
	if mode == "local" || mode == "windows" {
		if !which("docker") {
			return fmt.Errorf("missing tool: docker")
		}
	}
	return nil
}

func stackRunning(container string) bool {
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "--format", "{{.Names}}"})
	if err != nil || status != 0 {
		return false
	}
	for _, l := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(l) == container {
			return true
		}
	}
	return false
}

func ensureStack(f *setupSshFlags, mode string) error {
	if mode == "remote" {
		return nil
	}
	if stackRunning(f.container) {
		ui.Ok(fmt.Sprintf("Container '%s' is running.", f.container))
		return nil
	}
	ui.Warn(fmt.Sprintf("Container '%s' not running.", f.container))
	cwd, _ := os.Getwd()
	composePath := filepath.Join(cwd, f.composeFile)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("compose file not found: %s\nRun 'devcontainer-cli' first to generate it, or pass -f <path> to specify a different compose file", composePath)
	}
	proceed := true
	if !f.assumeYes {
		ok, err := prompt.Confirm("Start it now with docker compose up -d?", true)
		if err != nil {
			return err
		}
		proceed = ok
	}
	if !proceed {
		return fmt.Errorf("aborting — stack must be running")
	}
	if status, _ := docker.DockerCompose(f.composeFile, []string{"up", "-d"}, nil); status != 0 {
		return fmt.Errorf("docker compose up failed")
	}
	for tries := 20; tries > 0 && !stackRunning(f.container); tries-- {
		time.Sleep(time.Second)
	}
	if !stackRunning(f.container) {
		return fmt.Errorf("container failed to start")
	}
	return nil
}

func fetchPassword(f *setupSshFlags, mode string) {
	if mode == "remote" {
		return
	}
	ui.Log("Fetching temporary password from logs...")
	status, stdout, stderr, err := docker.DockerCapture([]string{"compose", "-f", f.composeFile, "logs", f.service})
	if err != nil || status != 0 {
		ui.Warn("Could not read logs.")
		return
	}
	combined := stdout + "\n" + stderr
	var last string
	for _, l := range strings.Split(combined, "\n") {
		if strings.Contains(l, f.user+" password") {
			last = l
		}
	}
	if last == "" {
		ui.Warn("Could not read password from logs (maybe key already installed).")
	} else {
		color.New(color.FgWhite).Printf("   %s\n", last)
	}
}

func genKey(keyPath string) error {
	if fileExists(keyPath) && fileExists(keyPath+".pub") {
		ui.Ok(fmt.Sprintf("Key already exists: %s", keyPath))
		return nil
	}
	ui.Log(fmt.Sprintf("Generating ed25519 key at %s", keyPath))
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	c := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("ssh-keygen failed")
	}
	ui.Ok("Key generated.")
	return nil
}

func containerIP(container string, f *setupSshFlags) (string, error) {
	format := `{{range $name, $net := .NetworkSettings.Networks}}{{$name}} {{$net.IPAddress}}{{"\n"}}{{end}}`
	status, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", format, container})
	if err != nil || status != 0 {
		return "", fmt.Errorf("could not resolve container IP")
	}
	type entry struct{ network, ip string }
	var entries []entry
	for _, l := range strings.Split(stdout, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		sep := strings.Index(l, " ")
		if sep < 0 {
			continue
		}
		ip := strings.TrimSpace(l[sep+1:])
		if ip != "" {
			entries = append(entries, entry{network: l[:sep], ip: ip})
		}
	}
	if len(entries) == 0 {
		return "", nil
	}
	if len(entries) == 1 {
		return entries[0].ip, nil
	}
	if f.assumeYes {
		ui.Warn(fmt.Sprintf("Container '%s' is on %d networks; using '%s' (%s).", container, len(entries), entries[0].network, entries[0].ip))
		return entries[0].ip, nil
	}
	choices := make([]prompt.Choice, len(entries))
	for i, e := range entries {
		choices[i] = prompt.Choice{Value: e.ip, Label: fmt.Sprintf("%s (%s)", e.network, e.ip)}
	}
	return prompt.Select("Container is on multiple networks. Select one:", choices, choices[0].Value)
}

type installResult struct {
	hostname string
	port     string
}

func installKey(f *setupSshFlags, mode string) (installResult, error) {
	pub, err := os.ReadFile(f.key + ".pub")
	if err != nil {
		return installResult{}, err
	}
	script := sshdefaults.AuthorizedKeysInstallScript()

	if mode == "remote" {
		if f.remote == "" {
			return installResult{}, fmt.Errorf("--remote USER@HOST required for remote mode")
		}
		ui.Log(fmt.Sprintf("Installing public key into %s via %s...", f.container, f.remote))
		remoteCmd := fmt.Sprintf("docker exec -i -u %s %s sh -c '%s'", f.user, f.container, script)
		c := exec.Command("ssh", f.remote, remoteCmd)
		c.Stdin = bytes.NewReader(pub)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return installResult{}, fmt.Errorf("remote key install failed")
		}
		return installResult{}, nil
	}

	if mode == "local" {
		ip, ierr := containerIP(f.container, f)
		if ierr != nil {
			return installResult{}, ierr
		}
		if ip == "" {
			return installResult{}, fmt.Errorf("could not resolve container IP")
		}
		ui.Log(fmt.Sprintf("Installing public key into %s (%s) via docker exec...", f.container, ip))
		if err := dockerExecStdin(pub, f.user, f.container, script); err != nil {
			return installResult{}, err
		}
		return installResult{hostname: ip}, nil
	}

	ui.Log(fmt.Sprintf("Installing public key into %s via docker exec...", f.container))
	if err := dockerExecStdin(pub, f.user, f.container, script); err != nil {
		return installResult{}, err
	}
	return installResult{hostname: "localhost", port: f.port}, nil
}

func dockerExecStdin(input []byte, user, container, script string) error {
	c := exec.Command("docker", "exec", "-i", "-u", user, container, "sh", "-c", script)
	c.Stdin = bytes.NewReader(input)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker exec key install failed")
	}
	return nil
}

func buildConfigBlock(mode string, f *setupSshFlags, inst installResult) (string, error) {
	return sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:      mode,
		Alias:     f.alias,
		User:      f.user,
		Key:       f.key,
		Hostname:  inst.hostname,
		Port:      inst.port,
		Remote:    f.remote,
		Container: f.container,
	})
}

var hostAliasRe = regexp.MustCompile(`^\s*Host\s+(.+)$`)

func aliasOfHostLine(line string) []string {
	m := hostAliasRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return strings.Fields(m[1])
}

func hasAliasBlock(content, alias string) bool {
	for _, line := range strings.Split(content, "\n") {
		if containsString(aliasOfHostLine(line), alias) {
			return true
		}
	}
	return false
}

func stripAliasBlock(content, alias string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(content, "\n") {
		hosts := aliasOfHostLine(line)
		isHostLine := len(hosts) > 0
		if skip {
			if isHostLine {
				if containsString(hosts, alias) {
					continue
				}
				skip = false
				out = append(out, line)
			}
			continue
		}
		if isHostLine && containsString(hosts, alias) {
			skip = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func extractAliasBlock(content, alias string) string {
	var out []string
	printing := false
	for _, line := range strings.Split(content, "\n") {
		hosts := aliasOfHostLine(line)
		isHostLine := len(hosts) > 0
		if printing {
			if isHostLine {
				if !containsString(hosts, alias) {
					break
				}
				out = append(out, line)
				continue
			}
			out = append(out, line)
			continue
		}
		if isHostLine && containsString(hosts, alias) {
			printing = true
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func updateSshConfig(f *setupSshFlags, mode string, inst installResult) error {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	configPath := filepath.Join(sshDir, "config")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if !fileExists(configPath) {
		if err := os.WriteFile(configPath, []byte(""), 0o600); err != nil {
			return err
		}
	}
	_ = os.Chmod(configPath, 0o600)

	newBlock, err := buildConfigBlock(mode, f, inst)
	if err != nil {
		return err
	}
	currentBytes, _ := os.ReadFile(configPath)
	current := string(currentBytes)

	if hasAliasBlock(current, f.alias) {
		ui.Warn(fmt.Sprintf("Host '%s' already defined in %s", f.alias, configPath))
		color.New(color.FgWhite).Println("---- existing ----")
		fmt.Println(extractAliasBlock(current, f.alias))
		color.New(color.FgWhite).Println("---- proposed ----")
		fmt.Println(newBlock)
		replace := true
		if !f.assumeYes {
			ok, cerr := prompt.Confirm(fmt.Sprintf("Replace existing block for Host '%s'?", f.alias), false)
			if cerr != nil {
				return cerr
			}
			replace = ok
		}
		if !replace {
			ui.Warn("Skipping ssh config update.")
			return nil
		}
		_ = os.WriteFile(configPath+".bak", currentBytes, 0o600)
		ui.Ok(fmt.Sprintf("Backup saved: %s.bak", configPath))
		stripped := strings.TrimRight(stripAliasBlock(current, f.alias), "\n")
		if stripped != "" {
			stripped += "\n\n"
		}
		if err := os.WriteFile(configPath, []byte(stripped+newBlock+"\n"), 0o600); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("Replaced Host '%s' in %s", f.alias, configPath))
	} else {
		body := strings.TrimRight(current, "\n")
		if body != "" {
			body += "\n\n"
		}
		if err := os.WriteFile(configPath, []byte(body+newBlock+"\n"), 0o600); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("Appended Host '%s' to %s", f.alias, configPath))
	}
	return nil
}

func testConnection(alias string) {
	ui.Log(fmt.Sprintf("Testing ssh %s ...", alias))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=2",
		"-o", "ServerAliveCountMax=2",
		alias, "echo OK",
	)
	out, err := c.Output()
	if ctx.Err() == context.DeadlineExceeded {
		ui.Warn(fmt.Sprintf("SSH test timed out after 15s. Try manually:  ssh %s", alias))
		return
	}
	if err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(l) == "OK" {
				ui.Ok(fmt.Sprintf("SSH alias '%s' works.", alias))
				return
			}
		}
	}
	ui.Warn(fmt.Sprintf("SSH test inconclusive. Try manually:  ssh %s", alias))
}

func deriveWorkspace(containerName string) string {
	cwd, _ := os.Getwd()
	if cfg, _ := domain.LoadConfig(cwd); cfg != nil && cfg.Workspace != "" {
		return cfg.Workspace
	}
	if containerName != "" {
		re := regexp.MustCompile("^(.+)-" + regexp.QuoteMeta(sshdefaults.ServiceName) + "$")
		if m := re.FindStringSubmatch(containerName); m != nil {
			return m[1]
		}
	}
	return domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer")
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
		f.composeFile = fmt.Sprintf(".dc_%s/build/docker-compose.yml", workspace)
	}
}

func runSetupSsh(cmd *cobra.Command, _ []string) error {
	f := collectSetupSshFlags(cmd)
	mode := detectMode(f)
	if err := checkPrereqs(mode); err != nil {
		return err
	}

	resolvedContainer := ""
	if mode != "remote" {
		target, err := resolveTargetService(f)
		if err != nil {
			return err
		}
		f.service = target.service
		f.container = target.container
		resolvedContainer = target.container
	}

	workspace := deriveWorkspace(resolvedContainer)
	applyWorkspaceDefaults(f, workspace)

	if !f.aliasExplicit && !f.assumeYes {
		alias, err := prompt.Input("SSH connection name (alias):", f.alias, func(v string) error {
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

	ui.Log(fmt.Sprintf("Mode: %s   Workspace: %s   Alias: %s   Key: %s", mode, workspace, f.alias, f.key))
	if mode != "remote" {
		ui.Log(fmt.Sprintf("Service: %s   Container: %s   Compose: %s", f.service, f.container, f.composeFile))
	}

	if err := ensureStack(f, mode); err != nil {
		return err
	}
	fetchPassword(f, mode)
	if err := genKey(f.key); err != nil {
		return err
	}
	inst, err := installKey(f, mode)
	if err != nil {
		return err
	}
	if err := updateSshConfig(f, mode, inst); err != nil {
		return err
	}
	testConnection(f.alias)
	ui.Ok(fmt.Sprintf("Done. Connect with:  ssh %s", f.alias))
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
