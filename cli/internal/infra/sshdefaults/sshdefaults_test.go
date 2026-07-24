package sshdefaults_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

func TestAuthorizedKeysInstallScript(t *testing.T) {
	got := sshdefaults.AuthorizedKeysInstallScript()
	// The home must be resolved from the container's live /etc/passwd (with a
	// /home/devuser fallback), never via ~: docker exec delivers a stale HOME=/
	// after the entrypoint's runtime UID remap, which turned ~/.ssh into //.ssh.
	for _, frag := range []string{`getent passwd "$(id -u)"`, "/home/devuser", `"$H/.ssh"`, "authorized_keys", "chmod 600"} {
		if !strings.Contains(got, frag) {
			t.Errorf("install script missing %q: %s", frag, got)
		}
	}
	if strings.Contains(got, "~/.ssh") {
		t.Errorf("install script must not rely on ~ expansion (stale HOME under docker exec): %s", got)
	}
}

func TestRemoteExportScript(t *testing.T) {
	block, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:      sshdefaults.ModeRemote,
		Alias:     "myws",
		User:      "devuser",
		KeyPath:   "~/.ssh/" + sshdefaults.KeyName,
		Remote:    "user@host",
		Container: "myws-devcontainer-ssh",
	})
	if err != nil {
		t.Fatalf("BuildConfigBlock: %v", err)
	}
	got := sshdefaults.RemoteExportScript(sshdefaults.RemoteExportOptions{
		PrivateKey:  []byte("PRIVATE-KEY-BYTES"),
		PublicKey:   []byte("ssh-ed25519 AAAA pub"),
		ConfigBlock: block,
	})

	keyPath := "~/.ssh/" + sshdefaults.KeyName
	for _, frag := range []string{
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh",
		"PRIVATE-KEY-BYTES",
		"ssh-ed25519 AAAA pub",
		"chmod 600 " + keyPath,
		"chmod 644 " + keyPath + ".pub",
		"cat >> ~/.ssh/config",
		"ProxyCommand ssh user@host",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("export script missing %q:\n%s", frag, got)
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
	for _, frag := range []string{"Host devcontainer", "HostName 172.20.0.2", "User devuser", "IdentityFile /home/u/.ssh/id_devcontainer", "IdentitiesOnly yes"} {
		if !strings.Contains(got, frag) {
			t.Errorf("local block missing %q:\n%s", frag, got)
		}
	}
}

func TestBuildConfigBlock_WorkspaceMarker(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "myalias",
		User:     "devuser",
		KeyPath:  "k",
		Hostname: "172.20.0.2",
		Kind:     sshdefaults.KindWorkspace,
		Ref:      "myproj",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantMarker := "# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myalias"
	if !strings.HasPrefix(got, wantMarker+"\n") {
		t.Errorf("expected block to start with marker %q:\n%s", wantMarker, got)
	}
	if !strings.Contains(got, "Host myalias") {
		t.Errorf("marked block missing Host stanza:\n%s", got)
	}
}

func TestBuildConfigBlock_ContainerMarker(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "dc-ssh",
		User:     "devuser",
		KeyPath:  "k",
		Hostname: "172.20.0.2",
		Kind:     sshdefaults.KindContainer,
		Ref:      "dc-ssh",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantMarker := "# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh"
	if !strings.HasPrefix(got, wantMarker+"\n") {
		t.Errorf("expected block to start with container marker %q:\n%s", wantMarker, got)
	}
}

func TestBuildConfigBlock_NoMarkerWhenKindEmpty(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "x",
		User:     "devuser",
		KeyPath:  "k",
		Hostname: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "devcontainer-cli:managed") {
		t.Errorf("did not expect a marker when Kind is empty:\n%s", got)
	}
	if !strings.HasPrefix(got, "Host x") {
		t.Errorf("expected stanza to start with Host:\n%s", got)
	}
}

func TestBuildConfigBlock_LocalRequiresHostname(t *testing.T) {
	if _, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{Mode: sshdefaults.ModeLocal, Alias: "x"}); err == nil {
		t.Error("expected error when hostname missing for local mode")
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
	if !strings.Contains(got, "IdentitiesOnly yes") {
		t.Errorf("expected IdentitiesOnly yes:\n%s", got)
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

func TestHostKeyOptions(t *testing.T) {
	opts := sshdefaults.HostKeyOptions("/cfg/ssh/known_hosts")
	want := map[string]string{
		"UserKnownHostsFile":    "/cfg/ssh/known_hosts",
		"StrictHostKeyChecking": "accept-new",
		// Unhashed, so the CLI can find and replace a rebuilt container's entry
		// by hostname.
		"HashKnownHosts": "no",
	}
	if len(opts) != len(want) {
		t.Fatalf("HostKeyOptions returned %d options, want %d: %+v", len(opts), len(want), opts)
	}
	for _, opt := range opts {
		if w, ok := want[opt.Keyword]; !ok || w != opt.Value {
			t.Errorf("unexpected option %s %s", opt.Keyword, opt.Value)
		}
		if got, wantLine := opt.Line(), "    "+opt.Keyword+" "+opt.Value; got != wantLine {
			t.Errorf("Line() = %q, want %q", got, wantLine)
		}
	}
}

// A devcontainer regenerates its host keys on every image rebuild while keeping
// the same IP, so its keys must be recorded in the CLI-managed known_hosts and
// never in the user's global one.
func TestBuildConfigBlock_LocalKnownHostsFile(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:           sshdefaults.ModeLocal,
		Alias:          "myws",
		User:           "devuser",
		KeyPath:        "k",
		Hostname:       "172.20.0.2",
		KnownHostsFile: "/cfg/ssh/known_hosts",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, frag := range []string{
		"    UserKnownHostsFile /cfg/ssh/known_hosts",
		"    StrictHostKeyChecking accept-new",
		"    HashKnownHosts no",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("local block missing %q:\n%s", frag, got)
		}
	}
}

func TestBuildConfigBlock_RemoteKnownHostsFile(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:           sshdefaults.ModeRemote,
		Alias:          "myws",
		User:           "devuser",
		KeyPath:        "k",
		Remote:         "user@host",
		Container:      "c",
		KnownHostsFile: sshdefaults.RemoteKnownHostsPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The block is pasted on another machine, so the path must be the ~ form.
	if !strings.Contains(got, "UserKnownHostsFile ~/.ssh/") {
		t.Errorf("remote block must point at a ~-relative known_hosts:\n%s", got)
	}
}

func TestBuildConfigBlock_NoHostKeyOptionsWhenUnset(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "x",
		User:     "devuser",
		KeyPath:  "k",
		Hostname: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "UserKnownHostsFile") || strings.Contains(got, "StrictHostKeyChecking") {
		t.Errorf("did not expect host-key options when KnownHostsFile is empty:\n%s", got)
	}
}
