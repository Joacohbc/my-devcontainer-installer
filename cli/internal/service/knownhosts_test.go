package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestParseHostKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "keeps type+blob and drops the trailing comment",
			in:   "ssh-ed25519 AAAAC3Nz root@abc123\nssh-rsa AAAAB3Nza root@abc123\n",
			want: []string{"ssh-ed25519 AAAAC3Nz", "ssh-rsa AAAAB3Nza"},
		},
		{
			name: "ignores noise and an unmatched glob",
			in:   "cat: /etc/ssh/ssh_host_*_key.pub: No such file\n\n",
			want: nil,
		},
		{
			name: "keeps ecdsa and security-key types",
			in:   "ecdsa-sha2-nistp256 AAAAE2Vj c\nsk-ssh-ed25519@openssh.com AAAAG c\n",
			want: []string{"ecdsa-sha2-nistp256 AAAAE2Vj", "sk-ssh-ed25519@openssh.com AAAAG"},
		},
		{
			name: "drops a truncated line with no blob",
			in:   "ssh-ed25519\n",
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseHostKeys(c.in); !slices.Equal(got, c.want) {
				t.Errorf("parseHostKeys() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestReplaceKnownHostsEntries(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		host        string
		keys        []string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "replaces the stale key of a rebuilt container",
			content:     "172.25.1.30 ssh-ed25519 OLDKEY\n",
			host:        "172.25.1.30",
			keys:        []string{"ssh-ed25519 NEWKEY"},
			wantContain: []string{"172.25.1.30 ssh-ed25519 NEWKEY"},
			wantAbsent:  []string{"OLDKEY"},
		},
		{
			name:        "leaves other hosts untouched",
			content:     "10.0.0.1 ssh-rsa KEEP\n172.25.1.30 ssh-ed25519 OLDKEY\n# a comment\n",
			host:        "172.25.1.30",
			keys:        []string{"ssh-ed25519 NEWKEY"},
			wantContain: []string{"10.0.0.1 ssh-rsa KEEP", "# a comment", "172.25.1.30 ssh-ed25519 NEWKEY"},
			wantAbsent:  []string{"OLDKEY"},
		},
		{
			name:        "matches the bracketed host:port and @marker spellings",
			content:     "[172.25.1.30]:22 ssh-ed25519 OLDKEY\n@revoked 172.25.1.30 ssh-rsa BAD\n",
			host:        "172.25.1.30",
			keys:        []string{"ssh-ed25519 NEWKEY"},
			wantContain: []string{"172.25.1.30 ssh-ed25519 NEWKEY"},
			wantAbsent:  []string{"OLDKEY", "BAD"},
		},
		{
			name:        "matches a host listed among comma-separated patterns",
			content:     "myws,172.25.1.30 ssh-ed25519 OLDKEY\n",
			host:        "172.25.1.30",
			keys:        []string{"ssh-ed25519 NEWKEY"},
			wantContain: []string{"172.25.1.30 ssh-ed25519 NEWKEY"},
			wantAbsent:  []string{"OLDKEY"},
		},
		{
			name:        "writes every key of a fresh file",
			content:     "",
			host:        "172.25.1.30",
			keys:        []string{"ssh-ed25519 A", "ssh-rsa B"},
			wantContain: []string{"172.25.1.30 ssh-ed25519 A\n172.25.1.30 ssh-rsa B\n"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ReplaceKnownHostsEntries(c.content, c.host, c.keys)
			for _, frag := range c.wantContain {
				if !strings.Contains(got, frag) {
					t.Errorf("result missing %q:\n%s", frag, got)
				}
			}
			for _, frag := range c.wantAbsent {
				if strings.Contains(got, frag) {
					t.Errorf("result should not contain %q:\n%s", frag, got)
				}
			}
			if strings.Contains(got, "\n\n") {
				t.Errorf("result should not grow blank lines:\n%q", got)
			}
		})
	}
}

func TestContainerHostKeys(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "ssh-ed25519 AAAAC3Nz root@box\n"}
	defer useFakeDocker(runner)()

	svc := SshService{Report: nopReporter{}}
	got, err := svc.ContainerHostKeys("ws-devcontainer-ssh")
	if err != nil {
		t.Fatalf("ContainerHostKeys: %v", err)
	}
	if want := []string{"ssh-ed25519 AAAAC3Nz"}; !slices.Equal(got, want) {
		t.Errorf("ContainerHostKeys() = %v, want %v", got, want)
	}
	call := runner.callContaining("exec")
	if call == nil {
		t.Fatal("expected a docker exec call")
	}
	if !slices.Contains(call, "ws-devcontainer-ssh") {
		t.Errorf("docker exec did not target the container: %v", call)
	}
	if !strings.Contains(strings.Join(call, " "), "/etc/ssh/ssh_host_") {
		t.Errorf("docker exec did not read the sshd host keys: %v", call)
	}
}

func TestContainerHostKeys_Errors(t *testing.T) {
	t.Run("docker failure", func(t *testing.T) {
		defer useFakeDocker(&fakeRunner{status: 1})()
		if _, err := (SshService{Report: nopReporter{}}).ContainerHostKeys("gone"); err == nil {
			t.Error("expected an error when the container cannot be exec'd")
		}
	})
	t.Run("no keys in output", func(t *testing.T) {
		defer useFakeDocker(&fakeRunner{status: 0, stdout: "\n"})()
		if _, err := (SshService{Report: nopReporter{}}).ContainerHostKeys("c"); err == nil {
			t.Error("expected an error when the container exposes no host keys")
		}
	})
}

// Pinning is what keeps a rebuilt container — new host keys, same IP — from
// tripping "REMOTE HOST IDENTIFICATION HAS CHANGED": the stale entry is
// rewritten from what docker reports, not left for the user to delete.
func TestPinContainerHostKeys(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	defer useFakeDocker(&fakeRunner{status: 0, stdout: "ssh-ed25519 NEWKEY root@box\n"})()

	knownHosts := domain.ManagedKnownHostsPath()
	if err := os.MkdirAll(filepath.Dir(knownHosts), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(knownHosts, []byte("172.25.1.30 ssh-ed25519 OLDKEY\n"), 0o600); err != nil {
		t.Fatalf("seed known_hosts: %v", err)
	}

	svc := SshService{Report: nopReporter{}}
	if err := svc.PinContainerHostKeys("ws-devcontainer-ssh", "172.25.1.30"); err != nil {
		t.Fatalf("PinContainerHostKeys: %v", err)
	}

	data, err := os.ReadFile(knownHosts)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if got := string(data); got != "172.25.1.30 ssh-ed25519 NEWKEY\n" {
		t.Errorf("known_hosts = %q, want the re-pinned key", got)
	}
}

func TestPinContainerHostKeys_CreatesFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defer useFakeDocker(&fakeRunner{status: 0, stdout: "ssh-ed25519 KEY root@box\n"})()

	svc := SshService{Report: nopReporter{}}
	if err := svc.PinContainerHostKeys("c", "172.25.1.30"); err != nil {
		t.Fatalf("PinContainerHostKeys: %v", err)
	}
	info, err := os.Stat(domain.ManagedKnownHostsPath())
	if err != nil {
		t.Fatalf("known_hosts not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("known_hosts mode = %v, want 0600", perm)
	}
}

func TestPinContainerHostKeys_RequiresHost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defer useFakeDocker(&fakeRunner{status: 0, stdout: "ssh-ed25519 KEY root@box\n"})()

	if err := (SshService{Report: nopReporter{}}).PinContainerHostKeys("c", "  "); err == nil {
		t.Error("expected an error when there is no host to pin against")
	}
}

func TestKnownHostsHosts(t *testing.T) {
	content := "172.25.1.30 ssh-ed25519 A\n" +
		"[172.25.2.30]:2222 ssh-ed25519 B\n" +
		"myws,172.25.3.30 ssh-rsa C\n" +
		"172.25.1.30 ssh-rsa D\n" + // second key for an already seen host
		"# comment\n\n"
	want := []string{"172.25.1.30", "172.25.2.30", "myws", "172.25.3.30"}
	if got := KnownHostsHosts(content); !slices.Equal(got, want) {
		t.Errorf("KnownHostsHosts() = %v, want %v", got, want)
	}
}

func TestRemoveKnownHostsEntries(t *testing.T) {
	content := "172.25.1.30 ssh-ed25519 A\n172.25.1.30 ssh-rsa B\n10.0.0.1 ssh-ed25519 KEEP\n"
	got, dropped := RemoveKnownHostsEntries(content, []string{"172.25.1.30", "missing"})
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2", dropped)
	}
	if got != "10.0.0.1 ssh-ed25519 KEEP\n" {
		t.Errorf("result = %q, want only the surviving host", got)
	}
}

// The sweep is keyed by address, so it treats a workspace block and a loose
// --container block identically — and also catches keys destroy left behind.
func TestOrphanHosts(t *testing.T) {
	knownHosts := "172.25.1.30 ssh-ed25519 WS\n" +
		"172.25.2.30 ssh-ed25519 LOOSE\n" +
		"172.25.9.99 ssh-ed25519 GONE\n"
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n" +
		"Host dc-ssh\n    HostName 172.25.2.30\n"

	want := []string{"172.25.9.99"}
	if got := orphanHosts(knownHosts, config); !slices.Equal(got, want) {
		t.Errorf("orphanHosts() = %v, want %v", got, want)
	}
}

func TestOrphanHosts_KeepsHandWrittenAndAliasTargets(t *testing.T) {
	knownHosts := "10.0.0.5 ssh-ed25519 HAND\nmybox ssh-ed25519 ALIAS\n"
	// A hand-written block pointing at the CLI's known_hosts, and a block with no
	// HostName (ssh dials the alias itself) — neither entry is orphaned.
	config := "Host tunnel\n    HostName 10.0.0.5\n\nHost mybox\n    User me\n"
	if got := orphanHosts(knownHosts, config); got != nil {
		t.Errorf("orphanHosts() = %v, want none", got)
	}
}

func TestOrphanHosts_NoConfigOrphansEverything(t *testing.T) {
	want := []string{"172.25.1.30"}
	if got := orphanHosts("172.25.1.30 ssh-ed25519 A\n", ""); !slices.Equal(got, want) {
		t.Errorf("orphanHosts() = %v, want %v", got, want)
	}
}

func TestForgetHostKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := domain.ManagedKnownHostsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("172.25.1.30 ssh-ed25519 A\n10.0.0.1 ssh-rsa KEEP\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	dropped, err := svc.ForgetHostKeys([]string{"172.25.1.30"})
	if err != nil || dropped != 1 {
		t.Fatalf("ForgetHostKeys = %d,%v, want 1,nil", dropped, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "10.0.0.1 ssh-rsa KEEP\n" {
		t.Errorf("known_hosts = %q, want only the surviving host", data)
	}

	// Nothing to do: no rewrite, no error.
	if dropped, err := svc.ForgetHostKeys([]string{"172.25.1.30"}); err != nil || dropped != 0 {
		t.Errorf("second ForgetHostKeys = %d,%v, want 0,nil", dropped, err)
	}
	if dropped, err := svc.ForgetHostKeys(nil); err != nil || dropped != 0 {
		t.Errorf("ForgetHostKeys(nil) = %d,%v, want 0,nil", dropped, err)
	}
}

func TestForgetHostKeysMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := SshService{Report: nopReporter{}}
	if dropped, err := svc.ForgetHostKeys([]string{"1.2.3.4"}); err != nil || dropped != 0 {
		t.Errorf("ForgetHostKeys with no file = %d,%v, want 0,nil", dropped, err)
	}
}

func TestOrphanKnownHosts_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setHomeDir(t, t.TempDir())
	svc := SshService{Report: nopReporter{}}
	got, err := svc.OrphanKnownHosts()
	if err != nil || got != nil {
		t.Errorf("OrphanKnownHosts with no known_hosts = %v,%v, want nil,nil", got, err)
	}
}
