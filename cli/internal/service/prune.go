package service

import (
	"os"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// PruneService finds and removes devcontainer-cli/* images. Selection and
// removal own the docker calls; the cli handles listing output and the
// confirmation prompt.
type PruneService struct {
	Report Reporter
}

// LocalImage is one devcontainer-cli image present in the local daemon.
type LocalImage struct {
	Ref string
	ID  string
}

func listCliImages() []LocalImage {
	status, stdout, _, err := docker.DockerCapture([]string{
		"images", "--filter", "reference=" + types.ImageNamespace + "/*",
		"--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalImage
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, LocalImage{Ref: strings.TrimSpace(parts[0]), ID: strings.TrimSpace(parts[1])})
	}
	return out
}

// SelectImages returns the images to remove and whether any cli images exist at
// all. With all=false only orphans (untracked, or whose project dir is gone)
// are selected; with all=true every cli image is selected.
func (s PruneService) SelectImages(all bool) (toRemove []LocalImage, anyExist bool) {
	images := listCliImages()
	if len(images) == 0 {
		return nil, false
	}
	if all {
		return images, true
	}

	tracked := map[string]bool{}
	live := map[string]bool{}
	for _, e := range domain.ListEntries() {
		tracked[e.Image] = true
		if _, err := os.Stat(e.ProjectDir); err == nil {
			live[e.Image] = true
		}
	}
	for _, img := range images {
		if !tracked[img.Ref] || !live[img.Ref] {
			toRemove = append(toRemove, img)
		}
	}
	return toRemove, true
}

// Remove deletes the given images, reporting each failure and returning the
// counts of removed and failed images.
func (s PruneService) Remove(images []LocalImage) (removed, failed int) {
	for _, img := range images {
		status, _ := docker.DockerInherit([]string{"rmi", img.Ref})
		if status == 0 {
			removed++
			continue
		}
		s.Report.Error("  ✗ Failed to remove %s (container may be running)", img.Ref)
		failed++
	}
	return removed, failed
}
