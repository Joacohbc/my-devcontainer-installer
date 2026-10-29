package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestValidateImageName(t *testing.T) {
	valid := []string{"data-stack", "my_image", "image1", "a", "web-dev-2"}
	for _, v := range valid {
		if err := domain.ValidateImageName(v); err != nil {
			t.Errorf("expected valid image name %q, got error: %v", v, err)
		}
	}

	invalid := []string{"-bad", "_bad", "Bad", "my image", "image@1", ""}
	for _, inv := range invalid {
		if err := domain.ValidateImageName(inv); err == nil {
			t.Errorf("expected invalid image name %q, got nil error", inv)
		}
	}
}

func TestImageDefLifecycle_SaveLoadResolveDelete(t *testing.T) {
	tmpDir := t.TempDir()

	def := domain.ImageDef{
		Name:   "custom-stack",
		Label:  "Custom Stack",
		Tag:    "v1.0.0",
		Source: domain.ImageSourceBuild,
		Modules: []types.SelectedModule{
			{ID: "nodejs"},
			{ID: "pnpm"},
		},
	}

	// Save
	if err := domain.SaveImageDef(def, tmpDir); err != nil {
		t.Fatalf("SaveImageDef failed: %v", err)
	}

	// Load
	defs, err := domain.LoadImageDefs(tmpDir)
	if err != nil {
		t.Fatalf("LoadImageDefs failed: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "custom-stack" {
		t.Fatalf("expected 1 def with name custom-stack, got %v", defs)
	}

	// Resolve
	resolved, ok, err := domain.ResolveImageDef("custom-stack", tmpDir)
	if err != nil || !ok {
		t.Fatalf("ResolveImageDef failed: ok=%v, err=%v", ok, err)
	}
	if resolved.Tag != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", resolved.Tag)
	}
	if tag := domain.ImageTag(resolved); tag != "devcontainer-cli-img/custom-stack:v1.0.0" {
		t.Errorf("unexpected image tag %s", tag)
	}

	// Delete
	if err := domain.DeleteImageDef("custom-stack", tmpDir); err != nil {
		t.Fatalf("DeleteImageDef failed: %v", err)
	}
	_, ok, _ = domain.ResolveImageDef("custom-stack", tmpDir)
	if ok {
		t.Errorf("expected custom-stack to be deleted")
	}
}

func TestCheckImageCompat(t *testing.T) {
	// Compatible image
	mockCompatCapture := func(args []string) (int, string, string) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, ".Config.Entrypoint") {
			return 0, `["/entrypoint.sh"]`, ""
		}
		if strings.Contains(cmdStr, types.LabelManaged) {
			return 0, "true\n", ""
		}
		return 0, "", ""
	}

	if err := domain.CheckImageCompat("test-img", mockCompatCapture); err != nil {
		t.Errorf("expected compatible image, got error: %v", err)
	}

	// Incompatible entrypoint
	mockIncompatEntry := func(args []string) (int, string, string) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, ".Config.Entrypoint") {
			return 0, `["/bin/bash"]`, ""
		}
		if strings.Contains(cmdStr, types.LabelManaged) {
			return 0, "true\n", ""
		}
		return 0, "", ""
	}
	if err := domain.CheckImageCompat("test-img", mockIncompatEntry); err == nil {
		t.Error("expected error for incompatible entrypoint, got nil")
	}

	// Missing label
	mockMissingLabel := func(args []string) (int, string, string) {
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, ".Config.Entrypoint") {
			return 0, `["/entrypoint.sh"]`, ""
		}
		if strings.Contains(cmdStr, types.LabelManaged) {
			return 0, "\n", ""
		}
		return 0, "", ""
	}
	if err := domain.CheckImageCompat("test-img", mockMissingLabel); err == nil {
		t.Error("expected error for missing managed label, got nil")
	}
}

func TestImageDefConfig(t *testing.T) {
	def := domain.ImageDef{
		Name:   "data-py",
		Tag:    "latest",
		Source: domain.ImageSourceBuild,
		Modules: []types.SelectedModule{
			{ID: "python"},
		},
	}
	cfg := domain.ImageDefConfig(def)
	if cfg.Mode != types.BuildModeCustom {
		t.Errorf("expected synthetic config mode to be custom, got %s", cfg.Mode)
	}
	if cfg.Image != "devcontainer-cli-img/data-py:latest" {
		t.Errorf("expected image devcontainer-cli-img/data-py:latest, got %s", cfg.Image)
	}
}

func TestIsLocalImage_ExcludesNamedImages(t *testing.T) {
	// Project image
	if !domain.IsLocalImage("devcontainer-cli/abc123def456:latest") {
		t.Errorf("expected project image to be classified as local")
	}
	// Named image
	if domain.IsLocalImage("devcontainer-cli-img/data-stack:latest") {
		t.Errorf("named image must NOT be classified as local project image")
	}
	if !domain.IsNamedImage("devcontainer-cli-img/data-stack:latest") {
		t.Errorf("expected devcontainer-cli-img/data-stack:latest to be classified as named image")
	}
}
