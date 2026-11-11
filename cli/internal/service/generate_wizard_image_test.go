package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func imageWizard(base *types.DevcontainerConfig) wizardContext {
	if base.Env == nil {
		base.Env = map[string]string{}
	}
	return wizardContext{
		base:      base,
		workspace: "ws",
		imageDefs: []domain.ImageDef{
			{Name: "android", Source: domain.ImageSourceBuild},
			{Name: "team-base", Source: domain.ImageSourceRemote, Ref: "ghcr.io/acme/base:1"},
		},
	}
}

func imageModeState(def string) *State {
	s := NewState()
	s.Set(stepKeyMode, string(types.BuildModeImage))
	if def != "" {
		s.Set(stepKeyImageDef, def)
	}
	return s
}

func choiceValues(f Field) []string {
	values := make([]string, 0, len(f.Choices))
	for _, c := range f.Choices {
		values = append(values, c.Value)
	}
	return values
}

func stepKeys(steps []Step) []string {
	keys := make([]string, 0, len(steps))
	for _, s := range steps {
		keys = append(keys, s.Key)
	}
	return keys
}

// The image mode is only worth offering when there is an image to pick.
func TestWizardModeStepOffersImageOnlyWithDefinitions(t *testing.T) {
	with := choiceValues(imageWizard(&types.DevcontainerConfig{}).modeStep().Build(NewState()))
	if !slices.Contains(with, string(types.BuildModeImage)) {
		t.Errorf("mode choices %v lack image although definitions exist", with)
	}
	without := choiceValues(wizardContext{base: &types.DevcontainerConfig{}}.modeStep().Build(NewState()))
	if slices.Contains(without, string(types.BuildModeImage)) {
		t.Errorf("mode choices %v offer image with no definition to pick", without)
	}
}

func TestWizardImageModeAsksForADefinitionInsteadOfAName(t *testing.T) {
	ctx := imageWizard(&types.DevcontainerConfig{})
	state := imageModeState("")
	if currentMode(state) != types.BuildModeImage {
		t.Fatalf("currentMode = %s, want image", currentMode(state))
	}

	keys := stepKeys(ctx.steps(state))
	if !slices.Contains(keys, stepKeyImageDef) {
		t.Errorf("steps %v lack the image definition picker", keys)
	}
	if slices.Contains(keys, stepKeyImage) {
		t.Error("image mode must not ask for a free-text image name")
	}
	if slices.Contains(keys, stepKeySkillsEnabled) {
		t.Error("image mode builds nothing, so it must not ask about skills")
	}
	for _, step := range ctx.categorySteps(state) {
		for _, v := range choiceValues(step.Build(state)) {
			if catalog.GetDockerfileModule(types.ModuleID(v)) != nil {
				t.Errorf("image mode offers module %q", v)
			}
		}
	}
}

func TestWizardImageModeReducesToTheDefinitionReference(t *testing.T) {
	cases := map[string]string{
		"android":   "devcontainer-cli-img/android:latest",
		"team-base": "ghcr.io/acme/base:1",
	}
	for def, want := range cases {
		t.Run(def, func(t *testing.T) {
			cfg := imageWizard(&types.DevcontainerConfig{}).reduce(imageModeState(def))
			if cfg.Mode != types.BuildModeImage || cfg.Image != want {
				t.Errorf("mode=%s image=%s, want image %s", cfg.Mode, cfg.Image, want)
			}
			if len(cfg.Dockerfile.Modules) != 0 || cfg.Dockerfile.From != "" {
				t.Errorf("image mode must not build: modules=%v from=%q", cfg.Dockerfile.Modules, cfg.Dockerfile.From)
			}
		})
	}
}

// Re-running the wizard on an image project starts on the image it uses.
func TestWizardImagePickerPreselectsTheProjectImage(t *testing.T) {
	ctx := imageWizard(&types.DevcontainerConfig{Mode: types.BuildModeImage, Image: "ghcr.io/acme/base:1"})
	field := ctx.imageDefStep().Build(imageModeState(""))
	if field.Initial != "team-base" {
		t.Errorf("initial = %v, want team-base", field.Initial)
	}
}

// Picking a build definition that was never built is refused on the spot,
// with the command that fixes it.
func TestWizardImagePickerRejectsAnUnbuiltDefinition(t *testing.T) {
	svc := newImageSvc(t)
	_ = svc.Create("android", CreateImageOpts{})
	defer useFakeDocker(&daemonRunner{local: map[string]bool{}})()

	ctx := imageWizard(&types.DevcontainerConfig{})
	ctx.images = svc
	validate := ctx.imageDefStep().Build(imageModeState("")).Validate
	if validate == nil {
		t.Fatal("the picker must validate the chosen definition")
	}
	if err := validate("android"); err == nil || !strings.Contains(err.Error(), "image build android") {
		t.Errorf("error = %v, want an image build hint", err)
	}
}

// A project already in image mode keeps that mode on offer even when no
// definition exists (its image is a registry reference), so reopening the
// wizard never silently turns it into a custom build.
func TestWizardModeStepKeepsImageForAnImageProject(t *testing.T) {
	ctx := wizardContext{base: &types.DevcontainerConfig{Mode: types.BuildModeImage, Image: "ghcr.io/acme/other:2"}, workspace: "ws"}
	field := ctx.modeStep().Build(NewState())
	if !slices.Contains(choiceValues(field), string(types.BuildModeImage)) {
		t.Fatalf("mode choices %v lack image for an image project", choiceValues(field))
	}
	if field.Initial != string(types.BuildModeImage) {
		t.Errorf("initial mode = %v, want image", field.Initial)
	}
}

// An image project whose reference matches no definition is offered its own
// image first, preselected, and keeping it leaves the project image unchanged.
func TestWizardImagePickerKeepsAnUnmatchedProjectImage(t *testing.T) {
	cases := map[string]wizardContext{
		"with definitions": imageWizard(&types.DevcontainerConfig{Mode: types.BuildModeImage, Image: "ghcr.io/acme/other:2"}),
		"without definitions": {
			base:      &types.DevcontainerConfig{Mode: types.BuildModeImage, Image: "ghcr.io/acme/other:2", Env: map[string]string{}},
			workspace: "ws",
		},
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			state := imageModeState("")
			if !slices.Contains(stepKeys(ctx.steps(state)), stepKeyImageDef) {
				t.Fatal("an image project must reach the image picker")
			}
			field := ctx.imageDefStep().Build(state)
			if len(field.Choices) == 0 || field.Choices[0].Value != field.Initial || !strings.Contains(field.Choices[0].Label, "ghcr.io/acme/other:2") {
				t.Fatalf("choices=%v initial=%v, want the current image first and preselected", field.Choices, field.Initial)
			}
			if err := field.Validate(field.Initial.(string)); err != nil {
				t.Errorf("keeping the current image must validate, got %v", err)
			}
			state.Set(stepKeyImageDef, field.Initial.(string))
			if cfg := ctx.reduce(state); cfg.Image != "ghcr.io/acme/other:2" || cfg.Mode != types.BuildModeImage {
				t.Errorf("mode=%s image=%s, want the project image kept", cfg.Mode, cfg.Image)
			}
		})
	}
}
