package commands

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

func runCommandWith(t *testing.T, flags map[string]string) *runOptions {
	t.Helper()
	cmd := newRunCommand()
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	opts, err := resolveRunOptions(cmd, service.RunService{Report: console})
	if err != nil {
		t.Fatalf("resolveRunOptions: %v", err)
	}
	return &opts
}

func runCommandError(t *testing.T, flags map[string]string) error {
	t.Helper()
	cmd := newRunCommand()
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := cmd.ValidateFlagGroups(); err != nil {
		return err
	}
	_, err := resolveRunOptions(cmd, service.RunService{Report: console})
	return err
}

func TestRunImageRunsTheDefinitionReference(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t, "devcontainer-cli-img/api:latest")

	opts := runCommandWith(t, map[string]string{flagImage: "api", flagNoInteractive: "true"})
	if opts.image != "devcontainer-cli-img/api:latest" {
		t.Errorf("image = %q, want the definition tag", opts.image)
	}
	if opts.variant != "api" || opts.containerName != "dc-api" {
		t.Errorf("source=%q name=%q, want api and dc-api", opts.variant, opts.containerName)
	}

	remote := runCommandWith(t, map[string]string{flagImage: "team-base", flagNoInteractive: "true"})
	if remote.image != "ghcr.io/acme/base:1" {
		t.Errorf("remote image = %q, want its registry reference", remote.image)
	}
}

func TestRunImageErrors(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t)

	cases := []struct {
		name    string
		flags   map[string]string
		wantErr string
	}{
		{"unbuilt definition", map[string]string{flagImage: "android", flagNoInteractive: "true"}, "devcontainer-cli image build android"},
		{"unknown definition", map[string]string{flagImage: "nope", flagNoInteractive: "true"}, "devcontainer-cli image list"},
		{"neither flag", map[string]string{flagNoInteractive: "true"}, "--profile or --image"},
		{"both flags", map[string]string{flagProfile: "base", flagImage: "api"}, "none of the others can be"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := runCommandError(t, c.flags)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestRunImageFlagCompletesImageNames(t *testing.T) {
	seedImageDefs(t)
	fn, ok := newRunCommand().GetFlagCompletionFunc(flagImage)
	if !ok {
		t.Fatal("run --image has no completion")
	}
	got, _ := fn(nil, nil, "a")
	if strings.Join(got, ",") != "android,api" {
		t.Errorf("completion = %v, want android,api", got)
	}
}
