package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// localImagesRunner fakes a daemon that holds exactly the given images, every
// one honouring the image contract.
type localImagesRunner struct {
	local map[string]bool
	calls [][]string
}

func (r *localImagesRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	r.calls = append(r.calls, args)
	if len(args) < 2 {
		return 0, "", ""
	}
	switch args[1] {
	case "images":
		if r.local[args[len(args)-1]] {
			return 0, "sha256:x", ""
		}
	case "inspect":
		switch {
		case strings.Contains(args[3], "Entrypoint"):
			return 0, `["/entrypoint.sh"]`, ""
		case strings.Contains(args[3], types.LabelManaged):
			return 0, "true", ""
		}
	}
	return 0, "", ""
}

func useLocalImages(t *testing.T, images ...string) *localImagesRunner {
	t.Helper()
	runner := &localImagesRunner{local: map[string]bool{}}
	for _, img := range images {
		runner.local[img] = true
	}
	docker.SetRunner(runner)
	docker.ResetDockerCache()
	t.Cleanup(func() {
		docker.ResetRunner()
		docker.ResetDockerCache()
	})
	return runner
}

func applyImageMode(t *testing.T, image string) (string, error) {
	t.Helper()
	cmd := NewRootCommand("dev")
	for k, v := range map[string]string{flagMode: "image", flagImage: image} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	flags, err := parseGenFlags(cmd)
	if err != nil {
		return "", err
	}
	config, err := resolveImageMode(t, flags, domain.DefaultConfig(t.TempDir()))
	return config.Image, err
}

// resolveImageMode runs the flag and validation stages a generation runs, the
// point where a mode=image project's image is resolved and verified.
func resolveImageMode(t *testing.T, flags *genFlags, config *types.DevcontainerConfig) (*types.DevcontainerConfig, error) {
	t.Helper()
	if err := applyGenFlags(config, flags); err != nil {
		return config, err
	}
	config.Compose.Subnet = ""
	flags.keepWorkspace = true
	return config, validateConfig(t.TempDir(), config, flags, service.GenerateService{})
}

func pulledRefs(r *localImagesRunner) []string {
	var refs []string
	for _, call := range r.calls {
		if len(call) > 2 && call[1] == "pull" {
			refs = append(refs, call[len(call)-1])
		}
	}
	return refs
}

// The wizard and a profile's image: leave the definition's reference in the
// config rather than its name; a remote one is still pulled and verified.
func TestModeImageVerifiesARemoteDefinitionReference(t *testing.T) {
	seedImageDefs(t)
	runner := useLocalImages(t)

	config := &types.DevcontainerConfig{Workspace: "ws", Mode: types.BuildModeImage, Image: "ghcr.io/acme/base:1"}
	if _, err := resolveImageMode(t, &genFlags{interactive: true}, config); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if refs := pulledRefs(runner); len(refs) != 1 || refs[0] != "ghcr.io/acme/base:1" {
		t.Errorf("pulled %v, want the remote definition verified once", refs)
	}
}

func TestModeImageResolvesTheImageDefinition(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t, "devcontainer-cli-img/api:latest")

	cases := []struct {
		image   string
		want    string
		wantErr string
	}{
		{image: "api", want: "devcontainer-cli-img/api:latest"},
		{image: "team-base", want: "ghcr.io/acme/base:1"},
		{image: "ghcr.io/acme/other:2", want: "ghcr.io/acme/other:2"},
		{image: "android", wantErr: "run 'devcontainer-cli image build android' first"},
	}
	for _, c := range cases {
		t.Run(c.image, func(t *testing.T) {
			got, err := applyImageMode(t, c.image)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error = %v, want it to mention %q", err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("image = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
