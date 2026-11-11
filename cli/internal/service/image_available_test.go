package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

// stripSource rewrites a definition's image.yml without its source line, the
// shape of a hand-written manifest.
func stripSource(t *testing.T, svc ImageService, name string) {
	t.Helper()
	path := filepath.Join(svc.ImagesDir, name, domain.ImageManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "source:") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A parent manifest without a source is a build definition, so a stale local
// image must still stop the derived build.
func TestImageBuildDerivedRejectsAnOutdatedParentWithoutSource(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("base", CreateImageOpts{Modules: []string{"github-cli", "python"}})
	_ = svc.Create("child", CreateImageOpts{From: "base", Modules: []string{"sqlite"}})
	stripSource(t, svc, "base")
	defer useFakeDocker(&daemonRunner{local: map[string]bool{"devcontainer-cli-img/base:latest": true}, modules: "github-cli"})()

	err := svc.Build("child")
	if err == nil || !strings.Contains(err.Error(), "image build base") {
		t.Fatalf("expected an out-of-date parent error, got %v", err)
	}
}

func TestImageAvailable(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("built", CreateImageOpts{})
	_ = svc.Create("unbuilt", CreateImageOpts{})
	_ = svc.Create("team", CreateImageOpts{RemoteRef: "ghcr.io/acme/devcontainer:1"})
	defer useFakeDocker(&daemonRunner{local: map[string]bool{"devcontainer-cli-img/built:latest": true}})()

	cases := []struct {
		name    string
		want    string
		wantErr string
	}{
		{name: "built", want: "devcontainer-cli-img/built:latest"},
		{name: "team", want: "ghcr.io/acme/devcontainer:1"},
		{name: "unbuilt", wantErr: "run 'devcontainer-cli image build unbuilt' first"},
		{name: "missing", wantErr: "devcontainer-cli image list"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := svc.Available(c.name)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("Available(%q) error = %v, want it to mention %q", c.name, err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("Available(%q) = %q, %v; want %q", c.name, got, err, c.want)
			}
		})
	}
}

// --mode image --image takes a definition name or a registry reference; only a
// definition is checked, a reference is handed to Docker as-is.
func TestImageRunnableRef(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("unbuilt", CreateImageOpts{})
	defer useFakeDocker(&daemonRunner{local: map[string]bool{}})()

	if got, err := svc.RunnableRef("ghcr.io/acme/devcontainer:1"); err != nil || got != "ghcr.io/acme/devcontainer:1" {
		t.Fatalf("registry reference = %q, %v; want it unchanged", got, err)
	}
	if _, err := svc.RunnableRef("unbuilt"); err == nil || !strings.Contains(err.Error(), "image build unbuilt") {
		t.Fatalf("unbuilt definition error = %v, want a build hint", err)
	}
}

func pulled(r *daemonRunner, ref string) bool {
	for _, call := range r.calls {
		if len(call) > 2 && call[1] == "pull" && call[len(call)-1] == ref {
			return true
		}
	}
	return false
}

// A remote definition is pulled and checked against the image contract before
// a container runs it; a build definition only has to be built.
func TestImageReadyVerifiesARemoteDefinition(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("built", CreateImageOpts{})
	_ = svc.Create("team", CreateImageOpts{RemoteRef: "ghcr.io/acme/devcontainer:1"})
	runner := &daemonRunner{local: map[string]bool{"devcontainer-cli-img/built:latest": true}}
	defer useFakeDocker(runner)()

	if got, err := svc.Ready("team"); err != nil || got != "ghcr.io/acme/devcontainer:1" {
		t.Fatalf("Ready(team) = %q, %v; want its reference", got, err)
	}
	if !pulled(runner, "ghcr.io/acme/devcontainer:1") {
		t.Errorf("a remote definition must be pulled before it runs; calls=%v", runner.calls)
	}
	if got, err := svc.Ready("built"); err != nil || got != "devcontainer-cli-img/built:latest" {
		t.Fatalf("Ready(built) = %q, %v; want its tag", got, err)
	}
	if pulled(runner, "devcontainer-cli-img/built:latest") {
		t.Error("a build definition is never pulled")
	}
}

func TestImageReadyRejectsAnIncompatibleRemoteImage(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("team", CreateImageOpts{RemoteRef: "ghcr.io/acme/devcontainer:1"})
	defer useFakeDocker(&fakeRunner{})()

	if _, err := svc.Ready("team"); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("error = %v, want the image contract failure", err)
	}
}

// Under mode=image the value may be a definition name or the reference a
// definition resolves to (what the wizard and a profile's image: store); both
// get the same checks. A malformed definition is reported, not run as a
// registry reference.
func TestImageRunnableRefChecksDefinitionsByNameOrReference(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("team", CreateImageOpts{RemoteRef: "ghcr.io/acme/devcontainer:1"})
	runner := &daemonRunner{local: map[string]bool{}}
	defer useFakeDocker(runner)()

	if got, err := svc.RunnableRef("ghcr.io/acme/devcontainer:1"); err != nil || got != "ghcr.io/acme/devcontainer:1" {
		t.Fatalf("RunnableRef(ref) = %q, %v", got, err)
	}
	if !pulled(runner, "ghcr.io/acme/devcontainer:1") {
		t.Errorf("a definition's reference must be verified like its name; calls=%v", runner.calls)
	}

	broken := filepath.Join(svc.ImagesDir, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, domain.ImageManifestFileName), []byte("name: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunnableRef("broken"); err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Fatalf("RunnableRef(broken) error = %v, want the manifest parse error", err)
	}
}
