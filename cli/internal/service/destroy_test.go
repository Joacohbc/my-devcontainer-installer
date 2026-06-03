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
	t.Setenv("APPDATA", tmp)

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
	t.Setenv("APPDATA", tmp)

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
	t.Setenv("APPDATA", tmp)

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

func TestDestroySkipsDownWhenNoComposeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

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
