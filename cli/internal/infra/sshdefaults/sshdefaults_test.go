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

func TestRemoteKeygenCommand(t *testing.T) {
	got := sshdefaults.RemoteKeygenCommand("~/.ssh/id_myws")
	for _, frag := range []string{"ssh-keygen", "-t ed25519", "-f ~/.ssh/id_myws", `-N ""`} {
		if !strings.Contains(got, frag) {
			t.Errorf("keygen command missing %q: %s", frag, got)
		}
	}
}

func TestRemoteInstallKeyCommand(t *testing.T) {
	got := sshdefaults.RemoteInstallKeyCommand("~/.ssh/id_myws", "user@host", "devuser", "myws-devcontainer-ssh")
	for _, frag := range []string{
		"cat ~/.ssh/id_myws.pub",
		"ssh user@host",
		"docker exec -i -u devuser myws-devcontainer-ssh",
		sshdefaults.AuthorizedKeysInstallScript(),
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("install command missing %q: %s", frag, got)
		}
	}
}

func TestBuildConfigBlock_Local(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "devcontainer",
		User:     "devuser",
		KeyPath:  "/home/u/.ssh/id_devcontainer",
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
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: sshdefaults.ModeLocal, Alias: "x"}); err == nil {
		t.Error("expected error when hostname missing for local mode")
	}
}

func TestBuildConfigBlock_WindowsDefaults(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:    sshdefaults.ModeWindows,
		Alias:   "devcontainer",
		User:    "devuser",
		KeyPath: "k",
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
		Mode:     sshdefaults.ModeWindows,
		Alias:    "devcontainer",
		User:     "devuser",
		KeyPath:  "k",
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
		Mode:      sshdefaults.ModeRemote,
		Alias:     "devcontainer",
		User:      "devuser",
		KeyPath:   "k",
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
	// The substitution must be deferred to the remote host (\$) and the format's
	// inner quotes escaped (\") so the ProxyCommand string is not broken.
	if !strings.Contains(got, `\$(docker inspect`) {
		t.Errorf("remote ProxyCommand must escape the command substitution as \\$:\n%s", got)
	}
	if !strings.Contains(got, `{{\"\n\"}}`) {
		t.Errorf("remote ProxyCommand must escape the template quotes as \\\":\n%s", got)
	}
	if strings.Contains(got, `{{"\n"}}`) {
		t.Errorf("remote ProxyCommand left unescaped template quotes:\n%s", got)
	}
}

func TestBuildConfigBlock_RemoteRequiresRemoteAndContainer(t *testing.T) {
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: sshdefaults.ModeRemote, Alias: "x", Container: "c"}); err == nil {
		t.Error("expected error when remote host missing")
	}
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: sshdefaults.ModeRemote, Alias: "x", Remote: "h"}); err == nil {
		t.Error("expected error when container missing")
	}
}

// An unrecognized mode is an error, not a half-rendered "Host x" stanza.
func TestBuildConfigBlock_UnknownMode(t *testing.T) {
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: "bogus", Alias: "x"}); err == nil {
		t.Error("expected error for unknown mode")
	}
}
