package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/project"
)

// resolveHostBuildIDs records the host user's UID/GID on the config so a
// local-cached image bakes them (matching the workspace owner without a runtime
// remap). They are transient — resolved fresh on every Plan, never persisted —
// and only set when not already provided (so callers/tests can override).
func resolveHostBuildIDs(config *types.DevcontainerConfig) {
	if config.BuildUID == 0 {
		config.BuildUID = os.Getuid()
	}
	if config.BuildGID == 0 {
		config.BuildGID = os.Getgid()
	}
}

// buildArgs returns the Dockerfile build args derived from the host ids, or nil
// when they are unset (the Dockerfile ARG defaults then apply). It is the single
// source for both the compose build block and the image fingerprint.
func buildArgs(config *types.DevcontainerConfig) map[string]string {
	if config.BuildUID <= 0 || config.BuildGID <= 0 {
		return nil
	}
	return map[string]string{
		"USER_UID": strconv.Itoa(config.BuildUID),
		"USER_GID": strconv.Itoa(config.BuildGID),
	}
}

// UsedSubnets returns the Docker subnets already in use, for conflict
// detection. Pass the networks that must not count as a conflict — a project's
// own network above all, which is already allocated with exactly the subnet the
// project is about to ask for.
func (s GenerateService) UsedSubnets(ignoreNetworks ...string) []domain.CidrRange {
	return domain.ListUsedSubnets(captureFunc(), ignoreNetworks...)
}

// NameConflicts returns managed-resource name clashes for the config.
func (s GenerateService) NameConflicts(config *types.DevcontainerConfig) []domain.Conflict {
	return domain.FindConflicts(config, docker.IsDockerAvailable(), captureFunc())
}

// EnsureSharedConfigVolume creates the shared tool-config volume if missing.
// It is best-effort: with docker unavailable it does nothing so generation of
// the project files never fails on a stopped daemon.
func (s GenerateService) EnsureSharedConfigVolume() error {
	if !docker.IsDockerAvailable() {
		return nil
	}
	return EnsureSharedConfigVolume(s.Report)
}

// GenerateService turns a resolved DevcontainerConfig into the rendered build
// artifacts and runs the optional build/pull. The cli layer resolves every
// decision (flags, wizard, confirmations) and feeds the finished config in;
// this service owns the rendering, fingerprinting and docker orchestration.
type GenerateService struct {
	Report Reporter
}

// GeneratePlan is the rendered output for a config, together with the image
// identity and whether a cached local image already satisfies it.
type GeneratePlan struct {
	Dockerfile     string
	Compose        string
	Env            string
	Image          string
	Fingerprint    string
	CachedImageHit bool
}

// Plan renders the Dockerfile, compose and env for the config. For local-cached
// builds with a Dockerfile it computes the content fingerprint, derives the
// image tag, and reports whether that image already exists locally. copyContents
// maps each referenced build-helper file to its on-disk content so the
// fingerprint reflects the scripts baked into the image.
func (s GenerateService) Plan(config *types.DevcontainerConfig, copyContents map[string]string) (GeneratePlan, error) {
	resolveHostBuildIDs(config)

	dockerfile, err := domain.GenerateDockerfile(config)
	if err != nil {
		return GeneratePlan{}, err
	}

	compose, err := domain.GenerateCompose(config)
	if err != nil {
		return GeneratePlan{}, err
	}

	plan := GeneratePlan{
		Dockerfile: dockerfile,
		Compose:    compose,
		Env:        domain.GenerateEnv(config),
		Image:      config.Image,
	}

	if config.Mode == types.BuildModeCustom && dockerfile != "" {
		moduleIDs := make([]string, 0, len(config.Dockerfile.Modules))
		for _, m := range config.Dockerfile.Modules {
			moduleIDs = append(moduleIDs, string(m.ID))
		}
		plan.Fingerprint = domain.ComputeFingerprint(dockerfile, copyContents, moduleIDs, buildArgs(config))
		plan.Image = domain.FingerprintTag(plan.Fingerprint)
		plan.CachedImageHit = domain.LocalImageExists(plan.Image, captureFunc())
	}

	return plan, nil
}

// Build runs "docker compose build" (or "pull" for remote images) for the
// generated compose file, reporting progress. It returns an error if the
// command exits non-zero.
func (s GenerateService) Build(composeFile string, remote bool) error {
	action := "build"
	if remote {
		action = "pull"
		s.Report.Info("Pulling image...")
	} else {
		s.Report.Info("Building...")
	}
	status, err := docker.DockerCompose(composeFile, []string{action}, nil)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker compose %s failed (exit %d)", action, status)
	}
	return nil
}

// MaterializeCustomScripts copies custom scripts defined in config to buildDir.
func (s GenerateService) MaterializeCustomScripts(config *types.DevcontainerConfig, buildDir string) ([]string, error) {
	names, err := domain.CollectCustomScriptFiles(config)
	if err != nil {
		return nil, err
	}
	for _, sc := range config.Dockerfile.Scripts {
		if sc.Source == "" {
			continue
		}
		data, rerr := domain.ReadCustomScript(sc)
		if rerr != nil {
			return nil, rerr
		}
		if werr := os.WriteFile(filepath.Join(buildDir, sc.BuildFile()), data, 0o755); werr != nil {
			return nil, werr
		}
	}
	return names, nil
}

// PrepareBuildDir sets up build directories, copies assets and returns file contents map.
func (s GenerateService) PrepareBuildDir(config *types.DevcontainerConfig, paths project.Paths) (map[string]string, []string, error) {
	buildDir := paths.BuildDir
	skipBuildArtifacts := config.Mode == types.BuildModeProfiles

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, nil, err
	}

	var copyFiles, postScriptFiles, customScripts []string
	if !skipBuildArtifacts {
		copyFiles, _ = domain.CollectRequiredCopyFiles(config)
		postScriptFiles, _ = domain.CollectRequiredPostScriptFiles(config)

		var err error
		customScripts, err = s.MaterializeCustomScripts(config, buildDir)
		if err != nil {
			return nil, nil, err
		}

		referenced := slices.Concat(copyFiles, postScriptFiles, customScripts)
		if missing := assets.ValidateRequiredFiles(referenced, buildDir); len(missing) > 0 {
			return nil, nil, fmt.Errorf("missing required script(s) (not embedded in binary or %s): %s", buildDir, strings.Join(missing, ", "))
		}

		if len(copyFiles) > 0 {
			pre := assets.Preflight(copyFiles, buildDir)
			if len(pre.Missing) > 0 {
				return nil, nil, fmt.Errorf("missing required scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
		if len(postScriptFiles) > 0 {
			pre := assets.Preflight(postScriptFiles, buildDir)
			if len(pre.Missing) > 0 {
				return nil, nil, fmt.Errorf("missing required post-install scripts: %s", strings.Join(pre.Missing, ", "))
			}
		}
	}

	copyContents := map[string]string{}
	for _, f := range slices.Concat(copyFiles, postScriptFiles, customScripts) {
		if data, rerr := os.ReadFile(filepath.Join(buildDir, f)); rerr == nil {
			copyContents[f] = string(data)
		}
	}

	if !skipBuildArtifacts {
		table := types.RenderSharedConfigTable()
		if werr := os.WriteFile(filepath.Join(buildDir, types.SharedConfigTableFileName), []byte(table), 0o644); werr != nil {
			return nil, nil, werr
		}
		copyContents[types.SharedConfigTableFileName] = table

		contextDoc, cerr := domain.GenerateContext(config)
		if cerr != nil {
			return nil, nil, cerr
		}
		if contextDoc != "" {
			if werr := os.WriteFile(filepath.Join(buildDir, types.ContextFileName), []byte(contextDoc), 0o644); werr != nil {
				return nil, nil, werr
			}
			copyContents[types.ContextFileName] = contextDoc
		}
	}

	return copyContents, postScriptFiles, nil
}

// WriteFiles writes the rendered Dockerfile, docker-compose.yml, and .env to disk.
func (s GenerateService) WriteFiles(paths project.Paths, plan *GeneratePlan, config *types.DevcontainerConfig) error {
	if plan.Dockerfile != "" {
		if err := os.WriteFile(paths.DockerfilePath, []byte(plan.Dockerfile), 0o644); err != nil {
			return err
		}
	}
	if plan.Compose != "" {
		if err := os.WriteFile(paths.ComposeFile, []byte(plan.Compose), 0o644); err != nil {
			return err
		}
	}
	if len(config.Env) > 0 || config.Compose.Subnet != "" {
		if err := os.WriteFile(paths.EnvPath, []byte(plan.Env), 0o644); err != nil {
			return err
		}
	}
	return nil
}
