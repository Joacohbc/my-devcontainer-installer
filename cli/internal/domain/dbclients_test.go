package domain

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func clientVersion(modules []types.SelectedModule, id types.ModuleID) string {
	for _, m := range modules {
		if m.ID == id {
			v, _ := m.Options["version"].(string)
			return v
		}
	}
	return "<absent>"
}

func TestMatchDBClientVersions(t *testing.T) {
	cases := []struct {
		name     string
		services []any
		modules  []types.SelectedModule
		client   types.ModuleID
		want     string
	}{
		{
			name:     "postgres service default version matches client",
			services: []any{"postgres"},
			modules:  []types.SelectedModule{{ID: types.ModulePostgresClient}},
			client:   types.ModulePostgresClient,
			want:     "18", // catalog default 18-alpine -> 18
		},
		{
			name:     "explicit postgres service version, alpine suffix stripped",
			services: []any{map[string]any{"id": "postgres", "options": map[string]any{"version": "16-alpine"}}},
			modules:  []types.SelectedModule{{ID: types.ModulePostgresClient, Options: map[string]any{"version": "auto"}}},
			client:   types.ModulePostgresClient,
			want:     "16",
		},
		{
			name:     "explicit client version is not overridden",
			services: []any{"postgres"},
			modules:  []types.SelectedModule{{ID: types.ModulePostgresClient, Options: map[string]any{"version": "18"}}},
			client:   types.ModulePostgresClient,
			want:     "18",
		},
		{
			name:     "client on auto with no matching service stays auto",
			services: []any{},
			modules:  []types.SelectedModule{{ID: types.ModulePostgresClient, Options: map[string]any{"version": "auto"}}},
			client:   types.ModulePostgresClient,
			want:     "auto",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &types.DevcontainerConfig{
				Dockerfile: types.DockerfileConfig{Modules: tc.modules},
				Compose:    types.ComposeConfig{Services: tc.services},
			}
			got := MatchDBClientVersions(cfg)
			if v := clientVersion(got, tc.client); v != tc.want {
				t.Errorf("client %s version = %q, want %q", tc.client, v, tc.want)
			}
		})
	}
}

// MatchDBClientVersions must not mutate the caller's config.
func TestMatchDBClientVersionsDoesNotMutateInput(t *testing.T) {
	cfg := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{Modules: []types.SelectedModule{
			{ID: types.ModulePostgresClient, Options: map[string]any{"version": "auto"}},
		}},
		Compose: types.ComposeConfig{Services: []any{"postgres"}},
	}
	_ = MatchDBClientVersions(cfg)
	if v, _ := cfg.Dockerfile.Modules[0].Options["version"].(string); v != "auto" {
		t.Errorf("input config was mutated: version = %q, want auto", v)
	}
}
