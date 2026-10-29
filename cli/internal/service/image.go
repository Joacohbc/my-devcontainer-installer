package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

// ImageService orchestrates operations on reusable, named devcontainer images.
type ImageService struct {
	Report    Reporter
	ImagesDir string
}

func (s ImageService) getImagesDir() string {
	if s.ImagesDir != "" {
		return s.ImagesDir
	}
	return domain.UserImageDir()
}

// ImageStatus represents the inspected state of a defined image.
type ImageStatus struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	FullTag     string `json:"fullTag"`
	Source      string `json:"source"`
	Ref         string `json:"ref,omitempty"`
	LocalExists bool   `json:"localExists"`
	Size        string `json:"size,omitempty"`
	Label       string `json:"label,omitempty"`
}

// CreateImageOpts configures the creation of a new ImageDef manifest.
type CreateImageOpts struct {
	FromProfile string
	RemoteRef   string
	Tag         string
	Label       string
}

// Create creates a new image definition manifest.
func (s ImageService) Create(name string, opts CreateImageOpts) error {
	if err := domain.ValidateImageName(name); err != nil {
		return err
	}

	def := domain.ImageDef{
		Name:   name,
		Label:  opts.Label,
		Tag:    opts.Tag,
		Source: domain.ImageSourceBuild,
	}
	if def.Tag == "" {
		def.Tag = "latest"
	}

	if opts.RemoteRef != "" {
		def.Source = domain.ImageSourceRemote
		def.Ref = opts.RemoteRef
	} else if opts.FromProfile != "" {
		prof, ok := catalog.Resolve(opts.FromProfile)
		if !ok {
			return fmt.Errorf("profile %q not found", opts.FromProfile)
		}
		for _, m := range prof.Modules {
			def.Modules = append(def.Modules, types.SelectedModule{ID: types.ModuleID(m)})
		}
		def.Scripts = prof.Scripts
		if def.Label == "" {
			def.Label = prof.Label
		}
	} else {
		// Default to base
		def.Modules = []types.SelectedModule{{ID: "github-cli"}}
	}

	dir := s.getImagesDir()
	if err := domain.SaveImageDef(def, dir); err != nil {
		return fmt.Errorf("saving image definition: %w", err)
	}

	s.Report.Success("Created image definition '%s' in %s", name, filepath.Join(dir, name))
	return nil
}

// List enumerates all registered image definitions and their local daemon status.
func (s ImageService) List() ([]ImageStatus, error) {
	defs, err := domain.LoadImageDefs(s.getImagesDir())
	if err != nil {
		return nil, err
	}

	statuses := make([]ImageStatus, 0, len(defs))
	for _, def := range defs {
		fullTag := domain.ImageTag(def)
		exists := domain.LocalImageExists(fullTag, captureFunc())
		var sizeStr string
		if exists {
			if _, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.Size}}", fullTag}); err == nil {
				sizeStr = strings.TrimSpace(stdout)
			}
		}

		statuses = append(statuses, ImageStatus{
			Name:        def.Name,
			Tag:         def.Tag,
			FullTag:     fullTag,
			Source:      string(def.Source),
			Ref:         def.Ref,
			LocalExists: exists,
			Size:        sizeStr,
			Label:       def.Label,
		})
	}

	return statuses, nil
}

// Info returns detailed status for a single named image.
func (s ImageService) Info(name string) (*ImageStatus, error) {
	def, ok, err := domain.ResolveImageDef(name, s.getImagesDir())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("image definition %q not found", name)
	}

	fullTag := domain.ImageTag(def)
	exists := domain.LocalImageExists(fullTag, captureFunc())
	var sizeStr string
	if exists {
		if _, stdout, _, err := docker.DockerCapture([]string{"inspect", "-f", "{{.Size}}", fullTag}); err == nil {
			sizeStr = strings.TrimSpace(stdout)
		}
	}

	return &ImageStatus{
		Name:        def.Name,
		Tag:         def.Tag,
		FullTag:     fullTag,
		Source:      string(def.Source),
		Ref:         def.Ref,
		LocalExists: exists,
		Size:        sizeStr,
		Label:       def.Label,
	}, nil
}

// Build compiles a local Dockerfile for the image definition and tags it.
func (s ImageService) Build(name string) error {
	def, ok, err := domain.ResolveImageDef(name, s.getImagesDir())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("image definition %q not found", name)
	}
	if def.Source == domain.ImageSourceRemote {
		return fmt.Errorf("image %q has source=remote; run 'devcontainer-cli image pull %s' instead", name, name)
	}

	targetTag := domain.ImageTag(def)
	s.Report.Info("Building image '%s' -> %s...", name, targetTag)

	tmpDir, err := os.MkdirTemp("", "devcontainer-img-build-*")
	if err != nil {
		return fmt.Errorf("creating temp build dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	config := domain.ImageDefConfig(def)
	paths := project.Paths{
		BuildDir:       tmpDir,
		DockerfilePath: filepath.Join(tmpDir, "Dockerfile"),
	}

	genSvc := GenerateService{Report: s.Report}
	copyContents, _, err := genSvc.PrepareBuildDir(config, paths)
	if err != nil {
		return fmt.Errorf("preparing build artifacts: %w", err)
	}

	plan, err := genSvc.Plan(config, copyContents)
	if err != nil {
		return fmt.Errorf("generating build plan: %w", err)
	}

	if err := os.WriteFile(paths.DockerfilePath, []byte(plan.Dockerfile), 0o644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	code, err := docker.DockerInherit([]string{"build", "-t", targetTag, tmpDir})
	if err != nil || code != 0 {
		return fmt.Errorf("docker build failed with exit code %d: %w", code, err)
	}

	if err := domain.CheckImageCompat(targetTag, captureFunc()); err != nil {
		s.Report.Warn("Compatibility check warning: %v", err)
	}

	s.Report.Success("Successfully built image %s", targetTag)
	return nil
}

// Pull downloads the remote image reference for the definition and verifies compatibility.
func (s ImageService) Pull(name string) error {
	def, ok, err := domain.ResolveImageDef(name, s.getImagesDir())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("image definition %q not found", name)
	}

	targetRef := def.Ref
	if targetRef == "" {
		targetRef = domain.ImageTag(def)
	}

	s.Report.Info("Pulling image '%s' (%s)...", name, targetRef)
	code, err := docker.DockerInherit([]string{"pull", targetRef})
	if err != nil || code != 0 {
		return fmt.Errorf("docker pull failed with exit code %d (if private, run 'docker login' first): %w", code, err)
	}

	if err := domain.CheckImageCompat(targetRef, captureFunc()); err != nil {
		return fmt.Errorf("pulled image contract verification failed: %w", err)
	}

	s.Report.Success("Successfully pulled and verified image %s", targetRef)
	return nil
}

// Export dumps the resolved Dockerfile, helper scripts, and instructions to targetDir.
func (s ImageService) Export(name string, targetDir string) error {
	def, ok, err := domain.ResolveImageDef(name, s.getImagesDir())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("image definition %q not found", name)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating export dir: %w", err)
	}

	config := domain.ImageDefConfig(def)
	paths := project.Paths{
		BuildDir:       targetDir,
		DockerfilePath: filepath.Join(targetDir, "Dockerfile"),
	}

	genSvc := GenerateService{Report: s.Report}
	copyContents, _, err := genSvc.PrepareBuildDir(config, paths)
	if err != nil {
		return fmt.Errorf("preparing assets: %w", err)
	}

	plan, err := genSvc.Plan(config, copyContents)
	if err != nil {
		return fmt.Errorf("planning Dockerfile: %w", err)
	}

	if err := os.WriteFile(paths.DockerfilePath, []byte(plan.Dockerfile), 0o644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	// Save image.yml
	if err := domain.SaveImageDef(def, targetDir); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	// Write BUILD.md
	buildMd := fmt.Sprintf(`# Building Image: %s

This directory contains the self-contained assets and Dockerfile to build
the reusable image %s.

## Build Command

`+"```bash"+`
docker build -t %s .
`+"```"+`

## Requirements
- Docker 20.10+
- Multi-arch build (optional): `+"`docker buildx build --platform linux/amd64,linux/arm64 -t %s .`"+`
`, def.Name, def.Name, domain.ImageTag(def), domain.ImageTag(def))

	if err := os.WriteFile(filepath.Join(targetDir, "BUILD.md"), []byte(buildMd), 0o644); err != nil {
		return fmt.Errorf("writing BUILD.md: %w", err)
	}

	s.Report.Success("Exported image %s to %s", name, targetDir)
	return nil
}

// Remove deletes the image definition and optionally removes the Docker image.
func (s ImageService) Remove(name string, deleteDockerImage bool) error {
	def, ok, err := domain.ResolveImageDef(name, s.getImagesDir())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("image definition %q not found", name)
	}

	if err := domain.DeleteImageDef(name, s.getImagesDir()); err != nil {
		return fmt.Errorf("deleting definition: %w", err)
	}

	if deleteDockerImage {
		tag := domain.ImageTag(def)
		s.Report.Info("Removing Docker image %s...", tag)
		_, _ = docker.DockerInherit([]string{"rmi", tag})
	}

	s.Report.Success("Removed image definition '%s'", name)
	return nil
}
