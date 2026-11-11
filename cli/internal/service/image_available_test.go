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
