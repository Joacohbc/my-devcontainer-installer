package commands

import (
	"fmt"
	"slices"
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
	if opts.source() != service.QuickRunSource(service.QuickRunImage, "api") {
		t.Errorf("quick-run source = %q, want image:api", opts.source())
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
		{"both flags", map[string]string{flagProfile: "base", flagImage: "api", flagNoInteractive: "true"}, "--profile and --image are mutually exclusive"},
		{"registry with an image", map[string]string{flagImage: "api", "registry": "ghcr.io/acme", flagNoInteractive: "true"}, "--registry applies only to --profile"},
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

// agent run shares run's code, so it must accept every flag that code names
// in its errors.
func TestAgentRunAcceptsImage(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t, "devcontainer-cli-img/api:latest")

	cmd := newAgentRunCommand()
	if _, ok := cmd.GetFlagCompletionFunc(flagImage); !ok {
		t.Fatal("agent run --image has no completion")
	}
	if err := cmd.Flags().Set(flagNoInteractive, "true"); err != nil {
		t.Fatal(err)
	}
	_, err := resolveRunOptions(cmd, service.RunService{Report: console})
	if err == nil || !strings.Contains(err.Error(), "--profile or --image") {
		t.Fatalf("error = %v, want both source flags named", err)
	}
	if err := cmd.Flags().Set(flagImage, "api"); err != nil {
		t.Fatalf("agent run must define --image: %v", err)
	}
	opts, err := resolveRunOptions(cmd, service.RunService{Report: console})
	if err != nil || opts.image != "devcontainer-cli-img/api:latest" {
		t.Fatalf("agent run --image api = %q, %v", opts.image, err)
	}
}

// selectPrompter answers each Select with the next scripted value, recording
// the choices it was offered.
type selectPrompter struct {
	fakeScriptedPrompter
	answers []string
	offered [][]service.Option
}

func (p *selectPrompter) Select(_ string, choices []service.Option, _ service.Option) (service.Option, error) {
	p.offered = append(p.offered, choices)
	answer := p.answers[0]
	p.answers = p.answers[1:]
	return service.Option{Value: answer}, nil
}

// The interactive picker labels definitions like the wizard does, and asks
// again when the chosen one cannot run yet instead of aborting the command.
func TestRunImagePickerReasksForAnUnbuiltDefinition(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t, "devcontainer-cli-img/api:latest")

	p := &selectPrompter{answers: []string{"image", "android", "api"}}
	report := &warnRecorder{}
	got, err := promptRunImage(p, report)
	if err != nil || got != "api" {
		t.Fatalf("picked %q, %v; want api after android was refused", got, err)
	}
	if len(report.warnings) != 1 || !strings.Contains(report.warnings[0], "image build android") {
		t.Errorf("warnings = %v, want the refusal reported through the given reporter", report.warnings)
	}
	if len(p.offered) != 3 {
		t.Fatalf("asked %d times, want source + two picks", len(p.offered))
	}
	labels := make([]string, 0, len(p.offered[1]))
	for _, c := range p.offered[1] {
		labels = append(labels, c.Label)
	}
	if !slices.Contains(labels, "api — API toolchain") {
		t.Errorf("image choices %v lack the definition label", labels)
	}
}

// warnRecorder is a Reporter that keeps its warnings.
type warnRecorder struct {
	warnings []string
}

func (r *warnRecorder) Info(string, ...any)    {}
func (r *warnRecorder) Success(string, ...any) {}
func (r *warnRecorder) Error(string, ...any)   {}
func (r *warnRecorder) Fatal(string, ...any)   {}
func (r *warnRecorder) Debug(string, ...any)   {}
func (r *warnRecorder) Warn(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

// --registry only applies to a profile, so an interactive run that sets it
// goes straight to the profile picker instead of offering image definitions
// it would then ignore --registry for.
func TestRunWithRegistrySkipsTheSourceQuestion(t *testing.T) {
	seedImageDefs(t)
	useLocalImages(t, "devcontainer-cli-img/api:latest")

	cmd := newRunCommand()
	if err := cmd.Flags().Set("registry", "ghcr.io/acme"); err != nil {
		t.Fatal(err)
	}
	p := &selectPrompter{answers: []string{"nodejs"}}
	kind, variant, _, err := resolveRunSource(cmd, true, p, &warnRecorder{})
	if err != nil || kind != service.QuickRunProfile || variant != "nodejs" {
		t.Fatalf("source = %s %s, %v; want profile nodejs", kind, variant, err)
	}
	if len(p.offered) != 1 || slices.ContainsFunc(p.offered[0], func(o service.Option) bool { return o.Value == service.QuickRunImage }) {
		t.Errorf("offered %v, want only the profile picker", p.offered)
	}
}
