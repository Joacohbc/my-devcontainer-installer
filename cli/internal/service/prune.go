package service

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// PruneService finds and removes CLI-managed docker resources (containers,
// images, networks and volumes) selected by the managed label. Selection is at
// the daemon level across every project: with all=true every managed resource
// is chosen, otherwise only the ones not currently in use are. Removal owns the
// docker calls; the cli handles listing output and the confirmation prompt.
type PruneService struct {
	Report Reporter
}

// managedFilter is the docker filter that scopes a query to resources this CLI
// created.
var managedFilter = "label=" + types.LabelManaged + "=true"

// ---------------------------------------------------------------------------
// Images
// ---------------------------------------------------------------------------

// LocalImage is one managed image present in the local daemon.
type LocalImage struct {
	Ref string
	ID  string
}

func listCliImages() []LocalImage {
	status, stdout, _, err := docker.DockerCapture([]string{
		"images", "--filter", managedFilter,
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

// imagesInUse returns the set of image references (and IDs) referenced by any
// container, running or stopped.
func imagesInUse() map[string]bool {
	status, stdout, _, err := docker.DockerCapture([]string{"ps", "-a", "--format", "{{.Image}}"})
	if err != nil || status != 0 {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if ref := strings.TrimSpace(line); ref != "" {
			set[ref] = true
		}
	}
	return set
}

// filterUnusedImages keeps only the images whose ref/ID is not referenced by a
// container.
func filterUnusedImages(images []LocalImage, inUse map[string]bool) []LocalImage {
	var out []LocalImage
	for _, img := range images {
		if inUse[img.Ref] || inUse[img.ID] {
			continue
		}
		out = append(out, img)
	}
	return out
}

// SelectImages returns the images to remove and whether any managed image
// exists. With all=false only images not referenced by any container are
// selected; with all=true every managed image is selected.
func (s PruneService) SelectImages(all bool) (toRemove []LocalImage, anyExist bool) {
	images := listCliImages()
	if len(images) == 0 {
		return nil, false
	}
	if all {
		return images, true
	}
	return filterUnusedImages(images, imagesInUse()), true
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

// ---------------------------------------------------------------------------
// Containers
// ---------------------------------------------------------------------------

// LocalContainer is one managed container present in the local daemon.
type LocalContainer struct {
	Name  string
	State string
}

func listCliContainers() []LocalContainer {
	status, stdout, _, err := docker.DockerCapture([]string{
		"ps", "-a", "--filter", managedFilter,
		"--format", "{{.Names}}\t{{.State}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalContainer
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		state := ""
		if len(parts) > 1 {
			state = strings.TrimSpace(parts[1])
		}
		out = append(out, LocalContainer{Name: name, State: state})
	}
	return out
}

// SelectContainers returns the containers to remove and whether any managed
// container exists. With all=false only containers that are not running are
// selected; with all=true every managed container is selected.
func (s PruneService) SelectContainers(all bool) (toRemove []LocalContainer, anyExist bool) {
	containers := listCliContainers()
	if len(containers) == 0 {
		return nil, false
	}
	if all {
		return containers, true
	}
	for _, c := range containers {
		if !strings.EqualFold(c.State, "running") {
			toRemove = append(toRemove, c)
		}
	}
	return toRemove, true
}

// RemoveContainers force-deletes the given containers, reporting each failure
// and returning the counts of removed and failed containers.
func (s PruneService) RemoveContainers(containers []LocalContainer) (removed, failed int) {
	for _, c := range containers {
		status, _ := docker.DockerInherit([]string{"rm", "-f", c.Name})
		if status == 0 {
			removed++
			continue
		}
		s.Report.Error("  ✗ Failed to remove container %s", c.Name)
		failed++
	}
	return removed, failed
}

// ---------------------------------------------------------------------------
// Networks
// ---------------------------------------------------------------------------

// LocalNetwork is one network created by this CLI.
type LocalNetwork struct {
	Name string
}

func listCliNetworks() []LocalNetwork {
	status, stdout, _, err := docker.DockerCapture([]string{
		"network", "ls", "--filter", managedFilter, "--format", "{{.Name}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalNetwork
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, LocalNetwork{Name: name})
		}
	}
	return out
}

// networksInUse returns the set of network names that have at least one attached
// container.
func networksInUse(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	args := append([]string{"network", "inspect", "-f", "{{.Name}} {{len .Containers}}"}, names...)
	status, stdout, _, err := docker.DockerCapture(args)
	if err != nil || status != 0 {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] != "0" {
			set[fields[0]] = true
		}
	}
	return set
}

// filterUnusedNetworks keeps only the networks with no attached container.
func filterUnusedNetworks(networks []LocalNetwork, inUse map[string]bool) []LocalNetwork {
	var out []LocalNetwork
	for _, n := range networks {
		if inUse[n.Name] {
			continue
		}
		out = append(out, n)
	}
	return out
}

// SelectNetworks returns the networks to remove and whether any managed network
// exists. With all=false only networks with no attached container are selected;
// with all=true every managed network is selected.
func (s PruneService) SelectNetworks(all bool) (toRemove []LocalNetwork, anyExist bool) {
	networks := listCliNetworks()
	if len(networks) == 0 {
		return nil, false
	}
	if all {
		return networks, true
	}
	names := make([]string, 0, len(networks))
	for _, n := range networks {
		names = append(names, n.Name)
	}
	return filterUnusedNetworks(networks, networksInUse(names)), true
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

// ---------------------------------------------------------------------------
// Volumes
// ---------------------------------------------------------------------------

// LocalVolume is one volume created by this CLI.
type LocalVolume struct {
	Name string
}

// listCliVolumes lists managed volumes. When unusedOnly is true the daemon's
// dangling filter restricts the result to volumes not referenced by any
// container.
func listCliVolumes(unusedOnly bool) []LocalVolume {
	args := []string{"volume", "ls", "--filter", managedFilter}
	if unusedOnly {
		args = append(args, "--filter", "dangling=true")
	}
	args = append(args, "--format", "{{.Name}}")
	status, stdout, _, err := docker.DockerCapture(args)
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []LocalVolume
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, LocalVolume{Name: name})
		}
	}
	return out
}

// SelectVolumes returns the volumes to remove and whether any managed volume
// exists. With all=false only volumes not referenced by any container are
// selected; with all=true every managed volume is selected.
func (s PruneService) SelectVolumes(all bool) (toRemove []LocalVolume, anyExist bool) {
	volumes := listCliVolumes(false)
	if len(volumes) == 0 {
		return nil, false
	}
	if all {
		return volumes, true
	}
	return listCliVolumes(true), true
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
