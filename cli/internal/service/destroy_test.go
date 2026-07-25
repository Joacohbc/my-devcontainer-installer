package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDestroyRemovesArtifactsAndDownsStack(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	projectDir := filepath.Join(tmp, ".dc_ws")
	composeFile := filepath.Join(projectDir, "build", "docker-compose.yml")
	configPath := filepath.Join(tmp, "devcontainer.config.json")
	if err := os.MkdirAll(filepath.Dir(composeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{composeFile, configPath} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := DestroyService{Report: nopReporter{}}
	err := svc.Run(DestroyTarget{
		Workspace:   "ws",
		ComposeFile: composeFile,
		ProjectDir:  projectDir,
		ConfigPath:  configPath,
		ProjectKey:  tmp,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if call := runner.callContaining("down"); call == nil {
		t.Errorf("expected a compose down call; calls=%v", runner.calls)
	}
	if fileExists(projectDir) {
		t.Error("project dir should have been removed")
	}
	if fileExists(configPath) {
		t.Error("config file should have been removed")
	}
}

func TestDestroyRemovesManagedSSHBlock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	sshConfig := filepath.Join(home, ".ssh", "config")
	const cfg = `# devcontainer-cli:managed workspace=ws
Host ws
    HostName 172.18.0.2
    User devuser

Host keepme
    HostName example.com
`
	if err := os.WriteFile(sshConfig, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	err := svc.Run(DestroyTarget{
		Workspace:   "ws",
		ComposeFile: filepath.Join(tmp, "missing", "docker-compose.yml"),
		ProjectDir:  filepath.Join(tmp, "missing"),
		ConfigPath:  filepath.Join(tmp, "devcontainer.config.json"),
		ProjectKey:  tmp,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, _ := os.ReadFile(sshConfig)
	got := string(data)
	if strings.Contains(got, "Host ws") || strings.Contains(got, "devcontainer-cli:managed") {
		t.Errorf("managed SSH block not removed:\n%s", got)
	}
	if !strings.Contains(got, "Host keepme") {
		t.Errorf("unrelated SSH block was removed:\n%s", got)
	}
	if _, err := os.Stat(sshConfig + ".bak"); err != nil {
		t.Errorf("expected ssh config backup: %v", err)
	}
}

func TestDestroyLeavesUntaggedSSHBlock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	sshConfig := filepath.Join(home, ".ssh", "config")
	const cfg = `Host ws
    HostName 172.18.0.2
`
	if err := os.WriteFile(sshConfig, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	if err := svc.Run(DestroyTarget{
		Workspace:   "ws",
		ComposeFile: filepath.Join(tmp, "missing", "docker-compose.yml"),
		ProjectDir:  filepath.Join(tmp, "missing"),
		ConfigPath:  filepath.Join(tmp, "devcontainer.config.json"),
		ProjectKey:  tmp,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, _ := os.ReadFile(sshConfig)
	if !strings.Contains(string(data), "Host ws") {
		t.Errorf("untagged block must be left intact:\n%s", string(data))
	}
}

func TestDestroyRunContainer(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	sshConfig := filepath.Join(home, ".ssh", "config")
	const cfg = `# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh
Host dc-ssh
    HostName 172.18.0.5
    User devuser

Host keepme
    HostName example.com
`
	if err := os.WriteFile(sshConfig, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	if err := svc.RunContainer("dc-ssh"); err != nil {
		t.Fatalf("RunContainer: %v", err)
	}

	if call := runner.callContaining("stop"); call == nil {
		t.Errorf("expected a docker stop call; calls=%v", runner.calls)
	}
	if call := runner.callContaining("rm"); call == nil {
		t.Errorf("expected a docker rm call; calls=%v", runner.calls)
	}

	data, _ := os.ReadFile(sshConfig)
	got := string(data)
	if strings.Contains(got, "Host dc-ssh") || strings.Contains(got, "devcontainer-cli:managed") {
		t.Errorf("managed SSH block not removed:\n%s", got)
	}
	if !strings.Contains(got, "Host keepme") {
		t.Errorf("unrelated SSH block was removed:\n%s", got)
	}
	if _, err := os.Stat(sshConfig + ".bak"); err != nil {
		t.Errorf("expected ssh config backup: %v", err)
	}
}

func TestDestroySkipsDownWhenNoComposeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	runner := &fakeRunner{status: 0}
	restore := useFakeDocker(runner)
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	err := svc.Run(DestroyTarget{
		Workspace:   "ws",
		ComposeFile: filepath.Join(tmp, "missing", "docker-compose.yml"),
		ProjectDir:  filepath.Join(tmp, "missing"),
		ConfigPath:  filepath.Join(tmp, "devcontainer.config.json"),
		ProjectKey:  tmp,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if call := runner.callContaining("down"); call != nil {
		t.Errorf("compose down must not run when no compose file exists; got %v", call)
	}
}

// destroy is the moment a workspace stops existing, so the host key its Host
// block had pinned goes with the block — no orphan left for a later 'clean ssh'.
func TestDestroyForgetsPinnedHostKey(t *testing.T) {
	tmp := t.TempDir()
	_, knownHostsPath := writeSSHFiles(t,
		"# devcontainer-cli:managed v=1 kind=workspace ref=ws alias=ws\n"+
			"Host ws\n    HostName 172.18.0.2\n    User devuser\n\n"+
			"Host keepme\n    HostName example.com\n",
		"172.18.0.2 ssh-ed25519 WS\nexample.com ssh-ed25519 KEEP\n")

	restore := useFakeDocker(&fakeRunner{status: 0})
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	if err := svc.Run(DestroyTarget{
		Workspace:   "ws",
		ComposeFile: filepath.Join(tmp, "missing", "docker-compose.yml"),
		ProjectDir:  filepath.Join(tmp, "missing"),
		ConfigPath:  filepath.Join(tmp, "devcontainer.config.json"),
		ProjectKey:  tmp,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "example.com ssh-ed25519 KEEP\n" {
		t.Errorf("known_hosts = %q, want only the key the surviving block dials", data)
	}
}

func TestDestroyRunContainerForgetsPinnedHostKey(t *testing.T) {
	_, knownHostsPath := writeSSHFiles(t,
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n"+
			"Host dc-ssh\n    HostName 172.18.0.5\n    User devuser\n",
		"172.18.0.5 ssh-ed25519 LOOSE\n")

	restore := useFakeDocker(&fakeRunner{status: 0})
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	if err := svc.RunContainer("dc-ssh"); err != nil {
		t.Fatalf("RunContainer: %v", err)
	}

	data, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("known_hosts = %q, want the pinned key dropped", data)
	}
}

// Two aliases can point at the same container; the key survives while anything
// still dials that address.
func TestDestroyKeepsHostKeyStillReferenced(t *testing.T) {
	knownHosts := "172.18.0.5 ssh-ed25519 LOOSE\n"
	_, knownHostsPath := writeSSHFiles(t,
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n"+
			"Host dc-ssh\n    HostName 172.18.0.5\n\n"+
			"Host my-shortcut\n    HostName 172.18.0.5\n",
		knownHosts)

	restore := useFakeDocker(&fakeRunner{status: 0})
	defer restore()

	svc := DestroyService{Report: nopReporter{}}
	if err := svc.RunContainer("dc-ssh"); err != nil {
		t.Fatalf("RunContainer: %v", err)
	}

	data, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != knownHosts {
		t.Errorf("known_hosts = %q, want the key kept for the surviving alias", data)
	}
}
