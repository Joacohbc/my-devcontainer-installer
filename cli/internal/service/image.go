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
	// Repo versions definition changes. Nil means the real configuration
	// repository when ImagesDir is the default, and no versioning when a custom
	// ImagesDir points elsewhere.
	Repo *ConfigRepo
}

func (s ImageService) getImagesDir() string {
	if s.ImagesDir != "" {
		return s.ImagesDir
	}
	return domain.UserImageDir()
}

// prepareRepo returns the repository to commit definition changes to, already
// prepared, or nil when the change is not versioned.
func (s ImageService) prepareRepo() *ConfigRepo {
	repo := s.Repo
	if repo == nil {
		if s.ImagesDir != "" {
			return nil
		}
		r := NewConfigRepo(reporterOrSilent(s.Report))
		repo = &r
	}
	if !repo.Prepare() {
		return nil
	}
	return repo
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
	// From derives the image from another definition or a registry reference.
	From      string
	Modules   []string
	RemoteRef string
	Tag       string
	Label     string
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

	if opts.From == name {
		return fmt.Errorf("image %q cannot derive from itself", name)
	}
	var scripts []types.CustomScript
	switch {
	case opts.RemoteRef != "":
		def.Source = domain.ImageSourceRemote
		def.Ref = opts.RemoteRef
	case opts.FromProfile != "":
		prof, ok := catalog.Resolve(opts.FromProfile, domain.ProfileDirs()...)
		if !ok {
			return fmt.Errorf("profile %q not found", opts.FromProfile)
		}
		if prof.Image != "" {
			def.From = prof.Image
		}
		def.Modules = selectedModules(prof.Modules)
		resolved, err := domain.ProfileScripts(prof)
		if err != nil {
			return err
		}
		scripts = resolved
		if def.Label == "" {
			def.Label = prof.Label
		}
	case opts.From == "" && len(opts.Modules) == 0:
		def.Modules = []types.SelectedModule{{ID: types.ModuleGithubCli}}
	}
	if opts.From != "" {
		def.From = opts.From
	}
	if len(opts.Modules) > 0 {
		def.Modules = selectedModules(opts.Modules)
	}
	for _, sc := range scripts {
		def.Scripts = append(def.Scripts, types.CustomScript{File: sc.File, When: sc.When})
	}

	dir := s.getImagesDir()
	_, existed, _ := domain.ResolveImageDef(name, dir)
	repo := s.prepareRepo()
	if err := domain.SaveImageDef(def, dir); err != nil {
		return fmt.Errorf("saving image definition: %w", err)
	}
	// The image is self-contained like a profile: its scripts live next to
	// image.yml, so the definition can be shared through the config repository.
	for _, sc := range scripts {
		data, err := domain.ReadCustomScript(sc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name, sc.File), data, 0o755); err != nil {
			return fmt.Errorf("copying script %s: %w", sc.File, err)
		}
	}
	if repo != nil {
		verb := "add"
		if existed {
			verb = "update"
		}
		repo.Commit(fmt.Sprintf("feat(image): %s %s", verb, name), filepath.Join(dir, name))
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

	config, err := s.buildConfig(def, true)
	if err != nil {
		return err
	}
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

	config, err := s.buildConfig(def, false)
	if err != nil {
		return err
	}
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

	tag := domain.ImageTag(def)
	if deleteDockerImage {
		if users := imageUsers(tag); len(users) > 0 {
			return fmt.Errorf("image %s is still used by %s; destroy those first with 'devcontainer-cli workspace destroy --yes', or keep the image with 'devcontainer-cli image remove %s --keep-docker-image'", tag, strings.Join(users, ", "), name)
		}
	}

	repo := s.prepareRepo()
	if err := domain.DeleteImageDef(name, s.getImagesDir()); err != nil {
		return fmt.Errorf("deleting definition: %w", err)
	}
	if repo != nil {
		repo.Commit("chore(image): remove "+name, filepath.Join(s.getImagesDir(), name))
	}
	s.Report.Success("Removed image definition '%s'", name)

	if !deleteDockerImage || !domain.LocalImageExists(tag, captureFunc()) {
		return nil
	}
	s.Report.Info("Removing Docker image %s...", tag)
	status, _, stderr, err := docker.DockerCapture([]string{"rmi", tag})
	if err != nil || status != 0 {
		return fmt.Errorf("the definition was removed but docker could not remove image %s: %s", tag, strings.TrimSpace(stderr))
	}
	return nil
}

// imageUsers names what still needs an image: registered projects that build
// on it and containers created from it.
func imageUsers(ref string) []string {
	var users []string
	for _, e := range domain.ListEntries() {
		if e.Image == ref {
			users = append(users, "project "+e.ProjectDir)
		}
	}
	if imagesInUse()[ref] {
		users = append(users, "a container")
	}
	return users
}

func selectedModules(ids []string) []types.SelectedModule {
	out := make([]types.SelectedModule, 0, len(ids))
	for _, id := range ids {
		out = append(out, types.SelectedModule{ID: types.ModuleID(id), Options: map[string]any{}})
	}
	return out
}

// buildConfig turns a definition into the config its Dockerfile is generated
// from: scripts resolved against the definition's directory and, for a derived
// image, the parent reference and the modules it already has. With
// requireParent the parent must exist locally (a registry parent is pulled).
func (s ImageService) buildConfig(def domain.ImageDef, requireParent bool) (*types.DevcontainerConfig, error) {
	config := domain.ImageDefConfig(def)
	dir := s.getImagesDir()
	for i := range config.Dockerfile.Scripts {
		config.Dockerfile.Scripts[i].Source = filepath.Join(dir, def.Name, config.Dockerfile.Scripts[i].File)
	}
	if def.From == "" {
		return config, nil
	}
	if requireParent {
		if err := s.ensureParent(def.From, dir); err != nil {
			return nil, err
		}
	}
	config.Dockerfile.From = def.From
	if err := resolveBaseImage(config, dir); err != nil {
		return nil, err
	}
	return config, nil
}

// ensureParent makes the parent image available: a definition must already be
// built (or pulled), a registry reference is pulled when missing. Either way
// it must honour the image contract.
func (s ImageService) ensureParent(from, dir string) error {
	ref := from
	parentDef, isDef, _ := domain.ResolveImageDef(from, dir)
	if isDef {
		ref = domain.ImageTag(parentDef)
	}
	if !domain.LocalImageExists(ref, captureFunc()) {
		if isDef {
			action := "build"
			if parentDef.Source == domain.ImageSourceRemote {
				action = "pull"
			}
			return fmt.Errorf("base image %s is not available locally; run 'devcontainer-cli image %s %s' first", ref, action, from)
		}
		s.Report.Info("Pulling base image %s...", ref)
		if code, err := docker.DockerInherit([]string{"pull", ref}); err != nil || code != 0 {
			return fmt.Errorf("pulling base image %s failed (exit %d)", ref, code)
		}
	}
	if err := domain.CheckImageCompat(ref, captureFunc()); err != nil {
		return fmt.Errorf("base image %s: %w", ref, err)
	}
	return nil
}

// resolveBaseImage fills a derived build's BaseRef and BaseModules from its
// From, reading image definitions from dir.
func resolveBaseImage(config *types.DevcontainerConfig, dir string) error {
	if config.Dockerfile.From == "" {
		return nil
	}
	parent, err := domain.ResolveImageParent(config.Dockerfile.From, captureFunc(), dir)
	if err != nil {
		return err
	}
	config.Dockerfile.BaseRef = parent.Ref
	config.Dockerfile.BaseModules = parent.Modules
	return nil
}

// baseImageID is the parent's content ID. It feeds a derived project's
// fingerprint, so rebuilding the parent under the same tag yields a new child.
func baseImageID(ref string) string {
	status, stdout, _ := captureFunc()([]string{"image", "inspect", "-f", "{{.Id}}", ref})
	if status != 0 {
		return ""
	}
	return strings.TrimSpace(stdout)
}
