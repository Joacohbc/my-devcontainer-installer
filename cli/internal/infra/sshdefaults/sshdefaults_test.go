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

// --via reuses ModeRemote's exact ProxyCommand stanza (Remote: the --via
// target): the container's IP is resolved fresh via `docker inspect` on every
// connection, never baked in as a static HostName, so a container recreated
// with a different address never breaks the alias.
func TestBuildConfigBlock_Via(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:      sshdefaults.ModeRemote,
		Alias:     "myalias",
		User:      "devuser",
		KeyPath:   "k",
		Remote:    "me@remote-host",
		Container: "myctr",
		Kind:      sshdefaults.KindContainer,
		Ref:       "myctr",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "HostName") {
		t.Errorf("--via block must not bake in a static HostName:\n%s", got)
	}
	for _, frag := range []string{"ProxyCommand ssh me@remote-host", `\$(docker inspect`} {
		if !strings.Contains(got, frag) {
			t.Errorf("--via block missing %q:\n%s", frag, got)
		}
	}
	wantMarker := "# devcontainer-cli:managed v=1 kind=container ref=myctr alias=myalias host=me@remote-host"
	if !strings.HasPrefix(got, wantMarker+"\n") {
		t.Errorf("expected marker to record the via host %q:\n%s", wantMarker, got)
	}
}

func TestBuildConfigBlock_LocalHasNoMarkerHost(t *testing.T) {
	got, err := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     sshdefaults.ModeLocal,
		Alias:    "myalias",
		User:     "devuser",
		KeyPath:  "k",
		Hostname: "172.20.0.2",
		Kind:     sshdefaults.KindContainer,
		Ref:      "myctr",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "host=") {
		t.Errorf("plain local block must not record a marker host:\n%s", got)
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

// One option, two grammars: the config stanza line and the command-line flag.
// This is the whole reason ConfigOption is data rather than a string in each
// call site.
func TestConfigOptionRendersInBothGrammars(t *testing.T) {
	option := sshdefaults.ConfigOption{Keyword: "StrictHostKeyChecking", Value: "accept-new"}

	if got, want := option.Line(), "    StrictHostKeyChecking accept-new"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
	if got := option.Flag(); len(got) != 2 || got[0] != "-o" || got[1] != "StrictHostKeyChecking=accept-new" {
		t.Errorf("Flag() = %v, want [-o StrictHostKeyChecking=accept-new]", got)
	}

	flags := sshdefaults.OptionFlags(sshdefaults.ProbeOptions())
	if len(flags) != 2*len(sshdefaults.ProbeOptions()) {
		t.Errorf("OptionFlags emitted %d flags for %d options", len(flags), len(sshdefaults.ProbeOptions()))
	}
	// A probe runs unattended: a prompt would hang it instead of answering.
	if !strings.Contains(strings.Join(flags, " "), "-o BatchMode=yes") {
		t.Errorf("ProbeOptions = %v, want BatchMode=yes", flags)
	}
}

// The two host-key policies must stay opposites: a managed block pins keys
// because something re-pins them after a rebuild, an ephemeral dial has no such
// machinery and must record nothing.
func TestHostKeyPoliciesAreDeliberatelyOpposite(t *testing.T) {
	managed := strings.Join(sshdefaults.OptionFlags(sshdefaults.HostKeyOptions("/managed/known_hosts")), " ")
	ephemeral := strings.Join(sshdefaults.EphemeralDialArgs("/keys/id_devcontainer"), " ")

	if !strings.Contains(managed, "StrictHostKeyChecking=accept-new") {
		t.Errorf("managed options = %q, want accept-new", managed)
	}
	if !strings.Contains(managed, "UserKnownHostsFile=/managed/known_hosts") {
		t.Errorf("managed options = %q, want the CLI-owned known_hosts", managed)
	}
	if strings.Contains(ephemeral, "accept-new") {
		t.Errorf("ephemeral args = %q, must not pin a host key", ephemeral)
	}
}

// The ephemeral dial flags are the contract two callers share (an ephemeral
// session and an ephemeral tunnel), so they are pinned here rather than in each.
func TestEphemeralDialArgs(t *testing.T) {
	args := strings.Join(sshdefaults.EphemeralDialArgs("/keys/id_devcontainer"), " ")

	// sshd runs inside the container on the stock port; a published host port
	// would not answer on the container's own address, which is what this dials.
	if !strings.Contains(args, "-p "+sshdefaults.Port) {
		t.Errorf("EphemeralDialArgs = %q, want it to dial port %s", args, sshdefaults.Port)
	}
	if sshdefaults.Port != "22" {
		t.Errorf("Port = %q, want 22 — the entrypoint starts sshd with no Port override", sshdefaults.Port)
	}
	if !strings.Contains(args, "-i /keys/id_devcontainer") {
		t.Errorf("EphemeralDialArgs = %q, want it to offer the given key", args)
	}
	// A container is recreated often and its host key changes with it, so an
	// ephemeral dial must not pin one: the managed-alias path owns that.
	for _, want := range []string{"StrictHostKeyChecking=no", "UserKnownHostsFile=/dev/null"} {
		if !strings.Contains(args, want) {
			t.Errorf("EphemeralDialArgs = %q, want %s", args, want)
		}
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
