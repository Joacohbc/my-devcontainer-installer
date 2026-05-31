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

// LocalNetwork is one network created by this CLI.
type LocalNetwork struct {
	Name    string
	Project string
}

// LocalVolume is one volume created by this CLI.
type LocalVolume struct {
	Name    string
	Project string
}

func listCliNetworks() []LocalNetwork {
	status, stdout, _, err := docker.DockerCapture([]string{
		"network", "ls", "--filter", "label=" + types.LabelManaged + "=true",
		"--format", "{{.Name}}\t{{.Label \"" + types.LabelProject + "\"}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalNetwork
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		proj := ""
		if len(parts) > 1 {
			proj = strings.TrimSpace(parts[1])
		}
		out = append(out, LocalNetwork{Name: strings.TrimSpace(parts[0]), Project: proj})
	}
	return out
}

func listCliVolumes() []LocalVolume {
	status, stdout, _, err := docker.DockerCapture([]string{
		"volume", "ls", "--filter", "label=" + types.LabelManaged + "=true",
		"--format", "{{.Name}}\t{{.Label \"" + types.LabelProject + "\"}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalVolume
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		proj := ""
		if len(parts) > 1 {
			proj = strings.TrimSpace(parts[1])
		}
		out = append(out, LocalVolume{Name: strings.TrimSpace(parts[0]), Project: proj})
	}
	return out
}

// SelectNetworks returns the networks to remove and whether any managed networks exist.
func (s PruneService) SelectNetworks(all bool) (toRemove []LocalNetwork, anyExist bool) {
	networks := listCliNetworks()
	if len(networks) == 0 {
		return nil, false
	}
	if all {
		return networks, true
	}

	liveProjectIDs := map[string]bool{}
	for _, e := range domain.ListEntries() {
		if _, err := os.Stat(e.ProjectDir); err == nil {
			replacer := strings.NewReplacer(":", "_", "/", "_", "@", "_")
			projectID := replacer.Replace(e.Image)
			liveProjectIDs[projectID] = true
		}
	}

	for _, net := range networks {
		if net.Project == "" || !liveProjectIDs[net.Project] {
			toRemove = append(toRemove, net)
		}
	}
	return toRemove, true
}

// SelectVolumes returns the volumes to remove and whether any managed volumes exist.
func (s PruneService) SelectVolumes(all bool) (toRemove []LocalVolume, anyExist bool) {
	volumes := listCliVolumes()
	if len(volumes) == 0 {
		return nil, false
	}
	if all {
		return volumes, true
	}

	liveProjectIDs := map[string]bool{}
	for _, e := range domain.ListEntries() {
		if _, err := os.Stat(e.ProjectDir); err == nil {
			replacer := strings.NewReplacer(":", "_", "/", "_", "@", "_")
			projectID := replacer.Replace(e.Image)
			liveProjectIDs[projectID] = true
		}
	}

	for _, vol := range volumes {
		if vol.Project == "" || !liveProjectIDs[vol.Project] {
			toRemove = append(toRemove, vol)
		}
	}
	return toRemove, true
}

// RemoveNetworks deletes the given networks.
func (s PruneService) RemoveNetworks(networks []LocalNetwork) (removed, failed int) {
	for _, net := range networks {
		status, _ := docker.DockerInherit([]string{"network", "rm", net.Name})
		if status == 0 {
			removed++
			continue
		}
		s.Report.Error("  ✗ Failed to remove network %s", net.Name)
		failed++
	}
	return removed, failed
}

// RemoveVolumes deletes the given volumes.
func (s PruneService) RemoveVolumes(volumes []LocalVolume) (removed, failed int) {
	for _, vol := range volumes {
		status, _ := docker.DockerInherit([]string{"volume", "rm", vol.Name})
		if status == 0 {
			removed++
			continue
		}
		s.Report.Error("  ✗ Failed to remove volume %s", vol.Name)
		failed++
	}
	return removed, failed
}
