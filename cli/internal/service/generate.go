package service

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// UsedSubnets returns the Docker subnets already in use, for conflict detection.
func (s GenerateService) UsedSubnets() []domain.CidrRange {
	return domain.ListUsedSubnets(captureFunc())
}

// NameConflicts returns managed-resource name clashes for the config.
func (s GenerateService) NameConflicts(config *types.DevcontainerConfig) []domain.Conflict {
	return domain.FindConflicts(config, docker.IsDockerAvailable(), captureFunc())
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

	if config.Mode == types.BuildModeLocalCached && dockerfile != "" {
		moduleIDs := make([]string, 0, len(config.Dockerfile.Modules))
		for _, m := range config.Dockerfile.Modules {
			moduleIDs = append(moduleIDs, string(m.ID))
		}
		plan.Fingerprint = domain.ComputeFingerprint(dockerfile, copyContents, moduleIDs)
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
