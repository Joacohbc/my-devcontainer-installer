package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var validImageNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type ImageSource string

const (
	ImageSourceBuild  ImageSource = "build"
	ImageSourceRemote ImageSource = "remote"
)

const ImageManifestFileName = "image.yml"

// ImageDef represents a reusable devcontainer image definition.
type ImageDef struct {
	Name       string                 `json:"name" yaml:"name"`
	Label      string                 `json:"label,omitempty" yaml:"label,omitempty"`
	Tag        string                 `json:"tag,omitempty" yaml:"tag,omitempty"`
	Source     ImageSource            `json:"source" yaml:"source"`
	Ref        string                 `json:"ref,omitempty" yaml:"ref,omitempty"`
	Modules    []types.SelectedModule `json:"modules,omitempty" yaml:"modules,omitempty"`
	Scripts    []types.CustomScript   `json:"scripts,omitempty" yaml:"scripts,omitempty"`
	Dockerfile string                 `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Files      []string               `json:"files,omitempty" yaml:"files,omitempty"`
}

// ValidateImageName checks if the image name matches the allowed character set.
func ValidateImageName(name string) error {
	if !validImageNameRe.MatchString(name) {
		return fmt.Errorf("invalid image name %q: must match ^[a-z0-9][a-z0-9_-]*$", name)
	}
	return nil
}

// UserImageDir returns the directory where user-defined image manifests are stored.
func UserImageDir(customBaseDir ...string) string {
	if len(customBaseDir) > 0 && customBaseDir[0] != "" {
		return filepath.Join(customBaseDir[0], "images")
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "devcontainer-cli", "images")
}

// ImageTag returns the Docker image tag for this image definition.
func ImageTag(def ImageDef) string {
	if def.Source == ImageSourceRemote && def.Ref != "" {
		return def.Ref
	}
	tag := def.Tag
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/%s:%s", types.ImageNamespaceImg, def.Name, tag)
}

// ImageDefConfig converts an ImageDef into a synthetic DevcontainerConfig
// for rendering its Dockerfile.
func ImageDefConfig(def ImageDef) *types.DevcontainerConfig {
	return &types.DevcontainerConfig{
		Mode:      types.BuildModeCustom,
		Image:     ImageTag(def),
		Workspace: "image",
		Dockerfile: types.DockerfileConfig{
			Modules: def.Modules,
			Scripts: def.Scripts,
		},
		Compose: types.ComposeConfig{},
	}
}

// LoadImageDefs reads all ImageDef definitions from the specified directory.
func LoadImageDefs(dir string) ([]ImageDef, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var defs []ImageDef
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), ImageManifestFileName)
		if data, err := os.ReadFile(manifestPath); err == nil {
			var def ImageDef
			if err := yaml.Unmarshal(data, &def); err == nil {
				if def.Name == "" {
					def.Name = entry.Name()
				}
				defs = append(defs, def)
			}
		}
	}
	return defs, nil
}

// ResolveImageDef looks up an image definition by name across given directories
// (defaulting to UserImageDir()).
func ResolveImageDef(name string, dirs ...string) (ImageDef, bool, error) {
	if err := ValidateImageName(name); err != nil {
		return ImageDef{}, false, err
	}
	searchDirs := dirs
	if len(searchDirs) == 0 {
		searchDirs = []string{UserImageDir()}
	}
	for _, dir := range searchDirs {
		manifestPath := filepath.Join(dir, name, ImageManifestFileName)
		if data, err := os.ReadFile(manifestPath); err == nil {
			var def ImageDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				return ImageDef{}, false, fmt.Errorf("parsing %s: %w", manifestPath, err)
			}
			if def.Name == "" {
				def.Name = name
			}
			return def, true, nil
		}
	}
	return ImageDef{}, false, nil
}

// SaveImageDef writes an image definition to the specified directory.
func SaveImageDef(def ImageDef, dir string) error {
	if err := ValidateImageName(def.Name); err != nil {
		return err
	}
	targetDir := filepath.Join(dir, def.Name)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, ImageManifestFileName), data, 0o644)
}

// DeleteImageDef removes the directory of an image definition.
func DeleteImageDef(name string, dir string) error {
	if err := ValidateImageName(name); err != nil {
		return err
	}
	targetDir := filepath.Join(dir, name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(targetDir)
}

// CheckImageCompat inspects a Docker image using capture to verify that it meets
// the required contract for devcontainer-cli (entrypoint is [/entrypoint.sh] and
// label dev.devcontainer-installer.managed is true).
func CheckImageCompat(ref string, capture CaptureFunc) error {
	status, stdout, stderr := capture([]string{"inspect", "-f", "{{json .Config.Entrypoint}}", ref})
	if status != 0 {
		return fmt.Errorf("inspecting image %q: %s", ref, strings.TrimSpace(stderr))
	}
	if !strings.Contains(stdout, "/entrypoint.sh") {
		return fmt.Errorf("image %q is incompatible: entrypoint must be [\"/entrypoint.sh\"], got %s", ref, strings.TrimSpace(stdout))
	}

	status, stdout, stderr = capture([]string{"inspect", "-f", "{{index .Config.Labels \"" + types.LabelManaged + "\"}}", ref})
	if status != 0 {
		return fmt.Errorf("inspecting labels on image %q: %s", ref, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(stdout) != "true" {
		return fmt.Errorf("image %q is incompatible: missing %s=\"true\" label", ref, types.LabelManaged)
	}

	return nil
}
