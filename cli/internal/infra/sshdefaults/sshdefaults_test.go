package sshdefaults_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

func TestDefaultKeyPath(t *testing.T) {
	got := sshdefaults.DefaultKeyPath()
	wantSuffix := filepath.Join(".ssh", sshdefaults.KeyName)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("DefaultKeyPath() = %q, want suffix %q", got, wantSuffix)
	}
}

func TestAuthorizedKeysInstallScript(t *testing.T) {
	got := sshdefaults.AuthorizedKeysInstallScript()
	for _, frag := range []string{"~/.ssh", "authorized_keys", "chmod 600"} {
		if !strings.Contains(got, frag) {
			t.Errorf("install script missing %q: %s", frag, got)
		}
	}
}

func TestBuildConfigBlock_Local(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     "local",
		Alias:    "devcontainer",
		User:     "devuser",
		Key:      "/home/u/.ssh/id_devcontainer",
		Hostname: "172.20.0.2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, frag := range []string{"Host devcontainer", "HostName 172.20.0.2", "User devuser", "IdentityFile /home/u/.ssh/id_devcontainer"} {
		if !strings.Contains(got, frag) {
			t.Errorf("local block missing %q:\n%s", frag, got)
		}
	}
}

func TestBuildConfigBlock_LocalRequiresHostname(t *testing.T) {
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: "local", Alias: "x"}); err == nil {
		t.Error("expected error when hostname missing for local mode")
	}
}

func TestBuildConfigBlock_WindowsDefaults(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:  "windows",
		Alias: "devcontainer",
		User:  "devuser",
		Key:   "k",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "HostName localhost") {
		t.Errorf("expected default hostname localhost:\n%s", got)
	}
	if !strings.Contains(got, "Port 2222") {
		t.Errorf("expected default windows port 2222:\n%s", got)
	}
}

func TestBuildConfigBlock_WindowsCustomHostnameAndPort(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     "windows",
		Alias:    "devcontainer",
		User:     "devuser",
		Key:      "k",
		Hostname: "192.168.1.5",
		Port:     "2200",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "HostName 192.168.1.5") || !strings.Contains(got, "Port 2200") {
		t.Errorf("custom hostname/port not honored:\n%s", got)
	}
}

func TestBuildConfigBlock_Remote(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:      "remote",
		Alias:     "devcontainer",
		User:      "devuser",
		Key:       "k",
		Remote:    "myhost",
		Container: "ws-devcontainer-ssh",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "ProxyCommand ssh myhost") {
		t.Errorf("remote block missing ProxyCommand:\n%s", got)
	}
	if !strings.Contains(got, "ws-devcontainer-ssh") {
		t.Errorf("remote block should reference the container:\n%s", got)
	}
}

func TestBuildConfigBlock_RemoteRequiresRemoteAndContainer(t *testing.T) {
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: "remote", Alias: "x", Container: "c"}); err == nil {
		t.Error("expected error when remote host missing")
	}
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: "remote", Alias: "x", Remote: "h"}); err == nil {
		t.Error("expected error when container missing")
	}
}
