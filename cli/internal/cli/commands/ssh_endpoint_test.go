package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

func TestStaleHostKeyName(t *testing.T) {
	ip := sshdefaults.Endpoint{Host: "172.25.0.14"}
	otherIP := sshdefaults.Endpoint{Host: "172.25.0.15"}
	published := sshdefaults.Endpoint{Host: "127.0.0.1", Port: "41234"}
	movedPublished := sshdefaults.Endpoint{Host: "127.0.0.1", Port: "50000"}

	cases := []struct {
		name           string
		previous, next sshdefaults.Endpoint
		want           string
	}{
		{"address changed", ip, otherIP, "172.25.0.14"},
		{"address to published port", ip, published, "172.25.0.14"},
		{"published port to address", published, ip, "myws"},
		{"published port moved, still keyed by the alias", published, movedPublished, ""},
	}
	for _, c := range cases {
		if got := staleHostKeyName("myws", c.previous, c.next); got != c.want {
			t.Errorf("%s: staleHostKeyName = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSameEndpoint(t *testing.T) {
	cases := []struct {
		name string
		a, b sshdefaults.Endpoint
		want bool
	}{
		{"same address", sshdefaults.Endpoint{Host: "10.0.0.2"}, sshdefaults.Endpoint{Host: "10.0.0.2"}, true},
		{"explicit stock port equals none", sshdefaults.Endpoint{Host: "10.0.0.2", Port: "22"}, sshdefaults.Endpoint{Host: "10.0.0.2"}, true},
		{"different port", sshdefaults.Endpoint{Host: "127.0.0.1", Port: "1"}, sshdefaults.Endpoint{Host: "127.0.0.1", Port: "2"}, false},
		{"different host", sshdefaults.Endpoint{Host: "10.0.0.2"}, sshdefaults.Endpoint{Host: "10.0.0.3"}, false},
	}
	for _, c := range cases {
		if got := sameEndpoint(c.a, c.b); got != c.want {
			t.Errorf("%s: sameEndpoint = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEndpointLabel(t *testing.T) {
	if got := endpointLabel(sshdefaults.Endpoint{Host: "172.25.0.14"}); got != "172.25.0.14" {
		t.Errorf("endpointLabel(address) = %q", got)
	}
	if got := endpointLabel(sshdefaults.Endpoint{Host: "127.0.0.1", Port: "41234"}); got != "127.0.0.1:41234" {
		t.Errorf("endpointLabel(published) = %q", got)
	}
}

func TestBuildConfigBlock_PublishedPortRendersPortAndKeyAlias(t *testing.T) {
	f := &setupSshFlags{alias: "myws", user: "devuser", key: "~/.ssh/id_myws", container: "myws-devcontainer-ssh"}
	block, err := buildConfigBlock(sshdefaults.ModeLocal, f, installResult{hostname: "127.0.0.1", port: "41234"}, sshdefaults.KindWorkspace, "myws")
	if err != nil {
		t.Fatalf("buildConfigBlock: %v", err)
	}
	for _, want := range []string{"HostName 127.0.0.1", "Port 41234", "HostKeyAlias myws"} {
		if !strings.Contains(block, want) {
			t.Errorf("published-port block missing %q:\n%s", want, block)
		}
	}
}

func TestBuildConfigBlock_ViaUsesTheEnginesBinaryInTheProxyCommand(t *testing.T) {
	f := &setupSshFlags{alias: "myws", user: "devuser", key: "k", via: "me@host", container: "myws-devcontainer-ssh"}
	block, err := buildConfigBlock(sshdefaults.ModeLocal, f, installResult{}, sshdefaults.KindWorkspace, "myws")
	if err != nil {
		t.Fatalf("buildConfigBlock: %v", err)
	}
	binary := service.EngineBinary()
	if !strings.Contains(block, binary+" inspect") {
		t.Errorf("--via block must inspect with %q on the remote host:\n%s", binary, block)
	}
}

func TestInstallResultEndpoint(t *testing.T) {
	published := installResult{hostname: "127.0.0.1", port: "41234"}.endpoint()
	if !published.IsPublished() {
		t.Errorf("%+v must count as a published endpoint", published)
	}
	if (installResult{hostname: "172.25.0.14"}).endpoint().IsPublished() {
		t.Error("an address with no port must not count as published")
	}
}

func TestRefreshLocalEndpoint_followsAContainerThatMovedToAPublishedPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".ssh", "devcontainer-cli.config")
	block := "# devcontainer-cli:managed v=1 kind=workspace ref=myws alias=myws\n" +
		"Host myws\n    HostName 172.25.0.14\n    User devuser\n"
	if err := os.WriteFile(configPath, []byte(block), 0o600); err != nil {
		t.Fatal(err)
	}
	ssh := service.SshService{Report: console}

	refreshLocalEndpoint(ssh, "myws", "myws-devcontainer-ssh", sshdefaults.Endpoint{Host: "127.0.0.1", Port: "41234"})

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"HostName 127.0.0.1", "Port 41234", "HostKeyAlias myws"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("refreshed block missing %q:\n%s", want, data)
		}
	}
}

func TestRefreshLocalEndpoint_leavesACurrentBlockAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".ssh", "devcontainer-cli.config")
	block := "# devcontainer-cli:managed v=1 kind=workspace ref=myws alias=myws\n" +
		"Host myws\n    HostName 172.25.0.14\n    User devuser\n"
	if err := os.WriteFile(configPath, []byte(block), 0o600); err != nil {
		t.Fatal(err)
	}
	ssh := service.SshService{Report: console}

	refreshLocalEndpoint(ssh, "myws", "myws-devcontainer-ssh", sshdefaults.Endpoint{Host: "172.25.0.14"})
	refreshLocalEndpoint(ssh, "myws", "myws-devcontainer-ssh", sshdefaults.Endpoint{})

	data, _ := os.ReadFile(configPath)
	if string(data) != block {
		t.Errorf("an up-to-date block was rewritten:\n%s", data)
	}
}
