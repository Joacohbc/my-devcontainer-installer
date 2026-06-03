package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadComposeServices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	body := "services:\n  devcontainer-ssh:\n    container_name: myws-devcontainer-ssh\n  db:\n    image: postgres\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	services := ReadComposeServices(path)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d (%v)", len(services), services)
	}
	if services["devcontainer-ssh"].ContainerName != "myws-devcontainer-ssh" {
		t.Errorf("container_name not parsed: %+v", services["devcontainer-ssh"])
	}

	if ReadComposeServices(filepath.Join(dir, "missing.yml")) != nil {
		t.Error("expected nil for a missing compose file")
	}
}

func TestContainerOf(t *testing.T) {
	if got := ContainerOf(ComposeService{ContainerName: "explicit"}, "key"); got != "explicit" {
		t.Errorf("ContainerOf explicit = %q, want explicit", got)
	}
	if got := ContainerOf(ComposeService{}, "key"); got != "key" {
		t.Errorf("ContainerOf fallback = %q, want key", got)
	}
}

func TestPickDevcontainerService(t *testing.T) {
	t.Run("exact service name", func(t *testing.T) {
		services := map[string]ComposeService{
			"devcontainer-ssh": {},
			"db":               {},
		}
		target, ok := PickDevcontainerService(services)
		if !ok || target.Service != "devcontainer-ssh" || target.Container != "devcontainer-ssh" {
			t.Errorf("got %+v ok=%v", target, ok)
		}
	})

	t.Run("unique heuristic match", func(t *testing.T) {
		services := map[string]ComposeService{
			"app": {ContainerName: "myws-devcontainer"},
			"db":  {},
		}
		target, ok := PickDevcontainerService(services)
		if !ok || target.Service != "app" || target.Container != "myws-devcontainer" {
			t.Errorf("got %+v ok=%v", target, ok)
		}
	})

	t.Run("ambiguous heuristic", func(t *testing.T) {
		services := map[string]ComposeService{
			"devcontainer-a": {},
			"devcontainer-b": {},
		}
		if _, ok := PickDevcontainerService(services); ok {
			t.Error("expected no pick when multiple candidates match")
		}
	})

	t.Run("no match", func(t *testing.T) {
		services := map[string]ComposeService{"db": {}, "cache": {}}
		if _, ok := PickDevcontainerService(services); ok {
			t.Error("expected no pick when nothing matches")
		}
	})
}
