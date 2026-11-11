package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// localImagesRunner fakes a daemon that holds exactly the given images.
type localImagesRunner struct {
	local map[string]bool
	calls [][]string
}

func (r *localImagesRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	r.calls = append(r.calls, args)
	if len(args) > 2 && args[1] == "images" && r.local[args[len(args)-1]] {
		return 0, "sha256:x", ""
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
	config := domain.DefaultConfig(t.TempDir())
	err = applyGenFlags(config, flags)
	return config.Image, err
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
