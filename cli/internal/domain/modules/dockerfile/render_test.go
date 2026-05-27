package dockerfile_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
)

func TestBaseModuleRender(t *testing.T) {
	out := dockerfile.BaseModule.Render(nil)
	if !strings.Contains(out, "FROM ubuntu:24.04") {
		t.Errorf("default base should pin ubuntu 24.04:\n%s", out)
	}
	out = dockerfile.BaseModule.Render(map[string]any{"ubuntu": "22.04"})
	if !strings.Contains(out, "FROM ubuntu:22.04") {
		t.Errorf("base should honor ubuntu option:\n%s", out)
	}
}

func TestNodejsModuleRender(t *testing.T) {
	cases := []struct {
		name    string
		opts    map[string]any
		want    []string
		notWant []string
	}{
		{
			name: "default nvm lts",
			opts: nil,
			want: []string{"NVM", "nvm install --lts"},
		},
		{
			name: "nvm pinned version",
			opts: map[string]any{"manager": "nvm", "version": "22"},
			want: []string{"nvm install 22"},
		},
		{
			name:    "fnm manager",
			opts:    map[string]any{"manager": "fnm"},
			want:    []string{"fnm", "fnm install --lts"},
			notWant: []string{"nvm install"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := dockerfile.NodejsModule.Render(tc.opts)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("expected output to contain %q:\n%s", w, out)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("expected output NOT to contain %q:\n%s", nw, out)
				}
			}
		})
	}
}

func TestPythonModuleRender(t *testing.T) {
	withUv := dockerfile.PythonModule.Render(map[string]any{"uv": true})
	if !strings.Contains(withUv, "astral.sh/uv") {
		t.Errorf("expected uv install block when uv=true:\n%s", withUv)
	}
	withoutUv := dockerfile.PythonModule.Render(map[string]any{"uv": false})
	if strings.Contains(withoutUv, "astral.sh/uv") {
		t.Errorf("did not expect uv block when uv=false:\n%s", withoutUv)
	}
	if !strings.Contains(withoutUv, "python3") {
		t.Errorf("python module must always install python3:\n%s", withoutUv)
	}
}

// Every Dockerfile module must render a non-empty fragment with default options
// and must not panic on a nil options map.
func TestAllDockerfileModulesRenderNonEmpty(t *testing.T) {
	for _, m := range catalog.DockerfileModules {
		if m.Render == nil {
			continue
		}
		out := m.Render(nil)
		if strings.TrimSpace(out) == "" {
			t.Errorf("module %q rendered an empty fragment", m.ID)
		}
	}
}
