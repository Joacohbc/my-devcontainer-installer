package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

// PruneService finds and removes CLI-managed docker resources (containers,
// images, networks and volumes) selected by the managed label, as well as
// stale catalog entries and SSH config blocks.
type PruneService struct {
	Report Reporter
	Prompt Prompter
	// IncludeSharedConfig allows the single shared tool-config volume
	// (devcontainer-shared-config) to be selected for removal.
	IncludeSharedConfig bool
}

type CleanOptions struct {
	DryRun      bool
	All         bool
	Yes         bool
	Interactive bool
}

// managedFilter is the docker filter that scopes a query to resources this CLI created.
var managedFilter = "label=" + types.LabelManaged + "=true"

func imageLabel(i LocalImage) string {
	return fmt.Sprintf("%s  (%s)", i.Ref, i.ID)
}

func containerLabel(c LocalContainer) string {
	if c.State != "" {
		return c.Name + "  (" + c.State + ")"
	}
	return c.Name
}

func selectByNames[T any](items []T, names []string, nameOf func(T) string) (selected []T, missing []string) {
	byName := make(map[string]T, len(items))
	for _, it := range items {
		byName[nameOf(it)] = it
	}
	for _, n := range names {
		if it, ok := byName[n]; ok {
			selected = append(selected, it)
		} else {
			missing = append(missing, n)
		}
	}
	return selected, missing
}

// CleanCatalog removes registry entries from images.json whose project directory no longer exists.
func (s PruneService) CleanCatalog(opts CleanOptions) ([]string, error) {
	entries := domain.ListEntries()
	var stale []string
	for _, e := range entries {
		if _, statErr := os.Stat(e.ProjectDir); os.IsNotExist(statErr) {
			stale = append(stale, e.ProjectDir)
		}
	}
	if len(stale) == 0 {
		s.Report.Success("No stale catalog entries found.")
		return nil, nil
	}

	s.Report.Info("Found %d stale catalog entry/entries:", len(stale))
	for _, dir := range stale {
		s.Report.Info("  - %s", dir)
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return stale, nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return nil, fmt.Errorf("cannot remove catalog entries in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these stale catalog entries?")
		if err != nil {
			return nil, err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return nil, nil
		}
	}

	var removed []string
	for _, dir := range stale {
		if domain.RemoveEntry(dir) {
			removed = append(removed, dir)
		}
	}
	s.Report.Success("Removed %d stale catalog entry/entries.", len(removed))
	return removed, nil
}

// CleanContainers removes CLI-managed containers.
func (s PruneService) CleanContainers(names []string, opts CleanOptions) (int, error) {
	var candidates []LocalContainer
	if len(names) > 0 {
		managed, _ := s.SelectContainers(true)
		selected, missing := selectByNames(managed, names, func(c LocalContainer) string { return c.Name })
		if len(missing) > 0 {
			return 0, fmt.Errorf("no managed containers found matching: %s", strings.Join(missing, ", "))
		}
		candidates = selected
	} else {
		selected, anyExist := s.SelectContainers(opts.All)
		if !anyExist {
			s.Report.Info("No devcontainer containers found locally.")
			return 0, nil
		}
		if len(selected) == 0 {
			s.Report.Info("No unused devcontainer containers found.")
			s.Report.Warn("Use --all to remove every managed containers.")
			return 0, nil
		}
		candidates = selected
	}

	s.Report.Warn("\ncontainers to remove (%d):", len(candidates))
	for _, c := range candidates {
		s.Report.Info("  %s", containerLabel(c))
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("cannot remove containers in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these containers?")
		if err != nil {
			return 0, err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
	}

	removed, failed := s.RemoveContainers(candidates)
	s.Report.Success("\nRemoved %d containers.", removed)
	if failed > 0 {
		s.Report.Error("%d failed.", failed)
	}
	return removed, nil
}

// CleanImages removes CLI-managed images.
func (s PruneService) CleanImages(refs []string, opts CleanOptions) (int, error) {
	var candidates []LocalImage
	if len(refs) > 0 {
		managed, _ := s.SelectImages(true)
		selected, missing := selectByNames(managed, refs, func(i LocalImage) string { return i.Ref })
		if len(missing) > 0 {
			return 0, fmt.Errorf("no managed images found matching: %s", strings.Join(missing, ", "))
		}
		candidates = selected
	} else {
		selected, anyExist := s.SelectImages(opts.All)
		if !anyExist {
			s.Report.Info("No devcontainer images found locally.")
			return 0, nil
		}
		if len(selected) == 0 {
			s.Report.Info("No unused devcontainer images found.")
			s.Report.Warn("Use --all to remove every managed images.")
			return 0, nil
		}
		candidates = selected
	}

	s.Report.Warn("\nimages to remove (%d):", len(candidates))
	for _, img := range candidates {
		s.Report.Info("  %s", imageLabel(img))
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("cannot remove images in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these images?")
		if err != nil {
			return 0, err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
	}

	removed, failed := s.Remove(candidates)
	s.Report.Success("\nRemoved %d images.", removed)
	if failed > 0 {
		s.Report.Error("%d failed.", failed)
	}
	return removed, nil
}

// CleanSSH prunes stale SSH config blocks from ~/.ssh/config, then drops the
// host keys those blocks had pinned in the CLI-managed known_hosts.
func (s PruneService) CleanSSH(opts CleanOptions) (int, error) {
	ssh := SshService{Report: s.Report}
	existsWorkspace, existsContainer, err := ssh.LiveTargetPredicates()
	if err != nil {
		return 0, err
	}

	stale, _, err := ssh.PruneManagedBlocks(existsWorkspace, existsContainer, true)
	if err != nil {
		return 0, err
	}
	if len(stale) == 0 {
		s.Report.Success("No stale SSH config blocks found.")
		// Keys can outlive their block: destroy removes the block on its own, and
		// a container coming back on another address leaves the old one pinned.
		return 0, s.cleanKnownHosts(ssh, opts)
	}

	s.Report.Info("Found %d stale SSH config block(s):", len(stale))
	for _, b := range stale {
		s.Report.Info("  - Host %s  (%s %s)", b.Alias, b.Kind, b.Ref)
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	toRemove := stale
	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("refusing to remove SSH config blocks without confirmation; pass --yes to confirm in non-interactive mode")
		}
		choices := make([]Option, len(stale))
		for i, b := range stale {
			choices[i] = Option{
				Value: strconv.Itoa(i),
				Label: fmt.Sprintf("Host %s  (%s %s)", b.Alias, b.Kind, b.Ref),
			}
		}
		picked, err := s.Prompt.Multiselect("Select SSH config blocks to remove:", choices, choices)
		if err != nil {
			return 0, err
		}
		if len(picked) == 0 {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
		toRemove = make([]ManagedMarker, len(picked))
		for i, p := range picked {
			idx, err := strconv.Atoi(p.Value)
			if err != nil {
				return 0, err
			}
			toRemove[i] = stale[idx]
		}
	}

	removed, backup, err := ssh.RemoveManagedBlocks(toRemove)
	if err != nil {
		return 0, err
	}
	if backup != "" {
		s.Report.Info("Backup saved: %s", backup)
	}
	s.Report.Success("Removed %d SSH config block(s) from ~/.ssh/config.", len(removed))

	// Runs after the blocks are gone, so the host keys they pinned are orphaned
	// by then and get swept in the same pass.
	if err := s.cleanKnownHosts(ssh, opts); err != nil {
		return len(removed), err
	}
	return len(removed), nil
}

// cleanKnownHosts drops the entries in the CLI-managed known_hosts that no Host
// block in ~/.ssh/config dials any more. It is keyed by address, not by marker,
// so it covers workspace and loose --container blocks alike — and also keys left
// behind by destroy, which removes a block without touching known_hosts.
func (s PruneService) cleanKnownHosts(ssh SshService, opts CleanOptions) error {
	orphans, err := ssh.OrphanKnownHosts()
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		return nil
	}

	s.Report.Info("Found %d orphaned host key entry(ies) in %s:", len(orphans), domain.ManagedKnownHostsPath())
	for _, host := range orphans {
		s.Report.Info("  - %s", host)
	}
	if opts.DryRun {
		s.Report.Warn("Dry run — host keys kept. Re-run without --dry-run to apply.")
		return nil
	}

	dropped, err := ssh.ForgetHostKeys(orphans)
	if err != nil {
		return err
	}
	s.Report.Success("Removed %d pinned host key(s).", dropped)
	return nil
}

// CleanNetworks removes CLI-managed Docker networks.
func (s PruneService) CleanNetworks(opts CleanOptions) (int, error) {
	candidates, anyExist := s.SelectNetworks(opts.All)
	if !anyExist {
		s.Report.Info("No devcontainer networks found locally.")
		return 0, nil
	}
	if len(candidates) == 0 {
		s.Report.Info("No unused devcontainer networks found.")
		s.Report.Warn("Use --all to remove every managed networks.")
		return 0, nil
	}

	s.Report.Warn("\nnetworks to remove (%d):", len(candidates))
	for _, n := range candidates {
		s.Report.Info("  %s", n.Name)
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("cannot remove networks in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these networks?")
		if err != nil {
			return 0, err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
	}

	removed, failed := s.RemoveNetworks(candidates)
	s.Report.Success("\nRemoved %d networks.", removed)
	if failed > 0 {
		s.Report.Error("%d failed.", failed)
	}
	return removed, nil
}

// CleanVolumes removes CLI-managed Docker volumes.
func (s PruneService) CleanVolumes(opts CleanOptions) (int, error) {
	candidates, anyExist := s.SelectVolumes(opts.All)
	if !anyExist {
		s.Report.Info("No devcontainer volumes found locally.")
		return 0, nil
	}
	if len(candidates) == 0 {
		s.Report.Info("No unused devcontainer volumes found.")
		s.Report.Warn("Use --all to remove every managed volumes.")
		return 0, nil
	}

	s.Report.Warn("\nvolumes to remove (%d):", len(candidates))
	for _, v := range candidates {
		s.Report.Info("  %s", v.Name)
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("cannot remove volumes in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these volumes?")
		if err != nil {
			return 0, err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
	}

	removed, failed := s.RemoveVolumes(candidates)
	s.Report.Success("\nRemoved %d volumes.", removed)
	if failed > 0 {
		s.Report.Error("%d failed.", failed)
	}
	return removed, nil
}

// CleanAll sweeps every category (catalog, containers, images, ssh, networks, volumes).
func (s PruneService) CleanAll(opts CleanOptions) error {
	sshSvc := SshService{Report: s.Report}

	var staleCatalog []string
	entries := domain.ListEntries()
	for _, e := range entries {
		if _, statErr := os.Stat(e.ProjectDir); os.IsNotExist(statErr) {
			staleCatalog = append(staleCatalog, e.ProjectDir)
		}
	}

	toRemoveContainers, anyContainers := s.SelectContainers(opts.All)
	toRemoveImages, anyImages := s.SelectImages(opts.All)

	var staleSSH []ManagedMarker
	existsWs, existsCnt, err := sshSvc.LiveTargetPredicates()
	if err == nil {
		staleSSH, _, _ = sshSvc.PruneManagedBlocks(existsWs, existsCnt, true)
	}

	toRemoveNetworks, anyNetworks := s.SelectNetworks(opts.All)
	toRemoveVolumes, anyVolumes := s.SelectVolumes(opts.All)

	totalItems := len(staleCatalog) + len(toRemoveContainers) + len(toRemoveImages) + len(staleSSH) + len(toRemoveNetworks) + len(toRemoveVolumes)

	if !anyContainers && !anyImages && !anyNetworks && !anyVolumes && len(staleCatalog) == 0 && len(staleSSH) == 0 {
		s.Report.Info("No devcontainer resources or stale entries found locally.")
		return nil
	}

	if totalItems == 0 {
		s.Report.Info("No unused devcontainer resources or stale entries found.")
		s.Report.Warn("Use --all to remove every managed resource.")
		return nil
	}

	if len(staleCatalog) > 0 {
		s.Report.Warn("\nStale catalog entries to remove (%d):", len(staleCatalog))
		for _, dir := range staleCatalog {
			s.Report.Info("  %s", dir)
		}
	}
	if len(toRemoveContainers) > 0 {
		s.Report.Warn("\nContainers to remove (%d):", len(toRemoveContainers))
		for _, c := range toRemoveContainers {
			s.Report.Info("  %s", containerLabel(c))
		}
	}
	if len(toRemoveImages) > 0 {
		s.Report.Warn("\nImages to remove (%d):", len(toRemoveImages))
		for _, img := range toRemoveImages {
			s.Report.Info("  %s  (%s)", img.Ref, img.ID)
		}
	}
	if len(staleSSH) > 0 {
		s.Report.Warn("\nStale SSH config blocks to remove (%d):", len(staleSSH))
		for _, b := range staleSSH {
			s.Report.Info("  Host %s  (%s %s)", b.Alias, b.Kind, b.Ref)
		}
	}
	if len(toRemoveNetworks) > 0 {
		s.Report.Warn("\nNetworks to remove (%d):", len(toRemoveNetworks))
		for _, net := range toRemoveNetworks {
			s.Report.Info("  %s", net.Name)
		}
	}
	if len(toRemoveVolumes) > 0 {
		s.Report.Warn("\nVolumes to remove (%d):", len(toRemoveVolumes))
		for _, vol := range toRemoveVolumes {
			s.Report.Info("  %s", vol.Name)
		}
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return nil
	}

	if !opts.Yes {
		if !opts.Interactive || s.Prompt == nil {
			return fmt.Errorf("cannot clean resources in non-interactive mode without --yes")
		}
		proceed, err := s.Prompt.Confirm("Remove these resources and stale entries?")
		if err != nil {
			return err
		}
		if !proceed {
			s.Report.Warn("Cancelled.")
			return nil
		}
	}

	var removedCatalogCount, removedSSHCount int
	for _, dir := range staleCatalog {
		if domain.RemoveEntry(dir) {
			removedCatalogCount++
		}
	}

	removedContainers, failedContainers := 0, 0
	if len(toRemoveContainers) > 0 {
		removedContainers, failedContainers = s.RemoveContainers(toRemoveContainers)
	}

	removedImages, failedImages := 0, 0
	if len(toRemoveImages) > 0 {
		removedImages, failedImages = s.Remove(toRemoveImages)
	}

	if len(staleSSH) > 0 {
		res, backup, err := sshSvc.RemoveManagedBlocks(staleSSH)
		if err == nil {
			removedSSHCount = len(res)
			if backup != "" {
				s.Report.Info("SSH backup saved: %s", backup)
			}
		}
	}

	removedNetworks, failedNetworks := 0, 0
	if len(toRemoveNetworks) > 0 {
		removedNetworks, failedNetworks = s.RemoveNetworks(toRemoveNetworks)
	}

	removedVolumes, failedVolumes := 0, 0
	if len(toRemoveVolumes) > 0 {
		removedVolumes, failedVolumes = s.RemoveVolumes(toRemoveVolumes)
	}

	removedTotal := removedCatalogCount + removedContainers + removedImages + removedSSHCount + removedNetworks + removedVolumes
	failedTotal := failedContainers + failedImages + failedNetworks + failedVolumes

	s.Report.Success("\nRemoved %d item(s).", removedTotal)
	if failedTotal > 0 {
		s.Report.Error("%d failed.", failedTotal)
	}
	return nil
}

// RunClean drives bare devcontainer-cli clean.
func (s PruneService) RunClean(opts CleanOptions) error {
	if opts.All || opts.Yes || !opts.Interactive || s.Prompt == nil {
		return s.CleanAll(opts)
	}

	options := []Option{
		{Value: "catalog", Label: "Stale catalog entries (images.json)"},
		{Value: "containers", Label: "Managed Docker containers"},
		{Value: "images", Label: "Managed Docker images"},
		{Value: "ssh", Label: "Stale SSH config blocks (~/.ssh/config)"},
		{Value: "networks", Label: "Managed Docker networks"},
		{Value: "volumes", Label: "Managed Docker volumes"},
		{Value: "all", Label: "All categories (sweep everything)"},
	}

	picked, err := s.Prompt.Multiselect("Select clean action(s) to run:", options, options)
	if err != nil {
		return err
	}
	if len(picked) == 0 {
		s.Report.Warn("Cancelled.")
		return nil
	}

	for _, p := range picked {
		if p.Value == "all" {
			return s.CleanAll(opts)
		}
	}

	for _, p := range picked {
		var subErr error
		switch p.Value {
		case "catalog":
			_, subErr = s.CleanCatalog(opts)
		case "containers":
			_, subErr = s.CleanContainers(nil, opts)
		case "images":
			_, subErr = s.CleanImages(nil, opts)
		case "ssh":
			_, subErr = s.CleanSSH(opts)
		case "networks":
			_, subErr = s.CleanNetworks(opts)
		case "volumes":
			_, subErr = s.CleanVolumes(opts)
		}
		if subErr != nil {
			return subErr
		}
	}
	return nil
}

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

func (s PruneService) SelectVolumes(all bool) (toRemove []LocalVolume, anyExist bool) {
	volumes := s.filterSharedConfig(listCliVolumes(false))
	if len(volumes) == 0 {
		return nil, false
	}
	if all {
		return volumes, true
	}
	return s.filterSharedConfig(listCliVolumes(true)), true
}

func (s PruneService) filterSharedConfig(volumes []LocalVolume) []LocalVolume {
	if s.IncludeSharedConfig {
		return volumes
	}
	var out []LocalVolume
	for _, v := range volumes {
		if v.Name == types.SharedConfigVolumeName {
			continue
		}
		out = append(out, v)
	}
	return out
}

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
