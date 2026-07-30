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

// removeResources runs `docker <args(item)>` for each item, counting successes
// and reporting each failure via onFail.
func removeResources[T any](items []T, args func(T) []string, onFail func(T)) (removed, failed int) {
	for _, it := range items {
		if status, _ := docker.DockerInherit(args(it)); status == 0 {
			removed++
			continue
		}
		onFail(it)
		failed++
	}
	return removed, failed
}

// captureLines runs `docker <args>` and returns its stdout as trimmed,
// non-empty lines (nil on error or empty output).
func captureLines(args []string) []string {
	status, stdout, _, err := docker.DockerCapture(args)
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// reportSelection prints the standard "nothing to do" messages for a resource
// category and returns whether there is anything to remove.
func (s PruneService) reportSelection(noun string, anyExist bool, count int) bool {
	if !anyExist {
		s.Report.Info("No devcontainer %s found locally.", noun)
		return false
	}
	if count == 0 {
		s.Report.Info("No unused devcontainer %s found.", noun)
		s.Report.Warn("Use --all to remove every managed %s.", noun)
		return false
	}
	return true
}

func (s PruneService) dryRunNotice() {
	s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
}

// confirmDestructive gates a removal: --yes proceeds; non-interactive without
// --yes returns nonInteractiveErr; otherwise it asks and reports cancellation.
func (s PruneService) confirmDestructive(opts CleanOptions, nonInteractiveErr error, prompt string) (bool, error) {
	if opts.Yes {
		return true, nil
	}
	if !opts.Interactive || s.Prompt == nil {
		return false, nonInteractiveErr
	}
	proceed, err := s.Prompt.Confirm(prompt)
	if err != nil {
		return false, err
	}
	if !proceed {
		s.Report.Warn("Cancelled.")
	}
	return proceed, nil
}

func (s PruneService) reportRemoval(noun string, removed, failed int) {
	s.Report.Success("\nRemoved %d %s.", removed, noun)
	if failed > 0 {
		s.Report.Error("%d failed.", failed)
	}
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
		if !s.reportSelection("containers", anyExist, len(selected)) {
			return 0, nil
		}
		candidates = selected
	}

	s.Report.Warn("\ncontainers to remove (%d):", len(candidates))
	for _, c := range candidates {
		s.Report.Info("  %s", containerLabel(c))
	}

	if opts.DryRun {
		s.dryRunNotice()
		return 0, nil
	}
	proceed, err := s.confirmDestructive(opts, fmt.Errorf("cannot remove containers in non-interactive mode without --yes"), "Remove these containers?")
	if err != nil {
		return 0, err
	}
	if !proceed {
		return 0, nil
	}

	removed, failed := s.RemoveContainers(candidates)
	s.reportRemoval("containers", removed, failed)
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
		if !s.reportSelection("images", anyExist, len(selected)) {
			return 0, nil
		}
		candidates = selected
	}

	s.Report.Warn("\nimages to remove (%d):", len(candidates))
	for _, img := range candidates {
		s.Report.Info("  %s", imageLabel(img))
	}

	if opts.DryRun {
		s.dryRunNotice()
		return 0, nil
	}
	proceed, err := s.confirmDestructive(opts, fmt.Errorf("cannot remove images in non-interactive mode without --yes"), "Remove these images?")
	if err != nil {
		return 0, err
	}
	if !proceed {
		return 0, nil
	}

	removed, failed := s.Remove(candidates)
	s.reportRemoval("images", removed, failed)
	return removed, nil
}

func viaOriginNote(b ManagedMarker) string {
	if b.Host == "" {
		return ""
	}
	return fmt.Sprintf("  [via %s — remote container, not managed on this machine's Docker]", b.Host)
}

// aliveSSHBlocks returns every managed block that is neither stale nor
// unverified — the --all category, useful to review or manually remove a
// healthy entry.
func (s PruneService) aliveSSHBlocks(ssh SshService, stale, unverified []ManagedMarker) ([]ManagedMarker, error) {
	all, err := ssh.ListManagedSSHBlocks()
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(stale)+len(unverified))
	for _, b := range stale {
		excluded[b.Kind+"|"+b.Ref] = true
	}
	for _, b := range unverified {
		excluded[b.Kind+"|"+b.Ref] = true
	}
	var alive []ManagedMarker
	for _, b := range all {
		if !excluded[b.Kind+"|"+b.Ref] {
			alive = append(alive, b)
		}
	}
	return alive, nil
}

// pickSSHBlocksToRemove asks the user which SSH config blocks to remove:
// stale ones pre-checked, unverified and alive ones offered but unchecked
// (removing either needs an explicit choice — see CleanSSH).
func (s PruneService) pickSSHBlocksToRemove(stale, unverified, alive []ManagedMarker) ([]ManagedMarker, error) {
	candidates := make([]ManagedMarker, 0, len(stale)+len(unverified)+len(alive))
	candidates = append(candidates, stale...)
	candidates = append(candidates, unverified...)
	candidates = append(candidates, alive...)
	unverifiedStart := len(stale)
	aliveStart := len(stale) + len(unverified)

	choices := make([]Option, len(candidates))
	initial := make([]Option, 0, len(stale))
	for i, b := range candidates {
		label := fmt.Sprintf("Host %s  (%s %s)%s", b.Alias, b.Kind, b.Ref, viaOriginNote(b))
		switch {
		case i >= aliveStart:
			label = fmt.Sprintf("Host %s  (%s %s)  [ALIVE]", b.Alias, b.Kind, b.Ref)
		case i >= unverifiedStart:
			label = fmt.Sprintf("Host %s  (%s %s)  [UNVERIFIED — could not reach %s]", b.Alias, b.Kind, b.Ref, b.Host)
		}
		choices[i] = Option{Value: strconv.Itoa(i), Label: label}
		if i < unverifiedStart {
			initial = append(initial, choices[i])
		}
	}

	picked, err := s.Prompt.Multiselect("Select SSH config blocks to remove:", choices, initial)
	if err != nil {
		return nil, err
	}
	toRemove := make([]ManagedMarker, len(picked))
	for i, p := range picked {
		idx, err := strconv.Atoi(p.Value)
		if err != nil {
			return nil, err
		}
		toRemove[i] = candidates[idx]
	}
	return toRemove, nil
}

// CleanSSH prunes stale SSH config blocks from the CLI-managed SSH config, then
// drops the host keys those blocks had pinned in the CLI-managed known_hosts.
//
// Managed blocks that older versions wrote directly into the user's
// ~/.ssh/config are lifted into the managed file first, so a clean run still
// finds and prunes them. A dry run skips that, since migrating is itself a write.
func (s PruneService) CleanSSH(opts CleanOptions) (int, error) {
	ssh := SshService{Report: s.Report}
	if !opts.DryRun {
		moved, merr := ssh.MigrateManagedBlocks()
		if merr != nil {
			return 0, merr
		}
		if len(moved) > 0 {
			s.Report.Info("Moved %d managed Host block(s) out of the user SSH config.", len(moved))
		}
	}
	existsWorkspace, existsContainer, existsRemoteContainer, err := ssh.LiveTargetPredicates()
	if err != nil {
		return 0, err
	}

	stale, unverified, _, err := ssh.PruneManagedBlocks(existsWorkspace, existsContainer, existsRemoteContainer, true)
	if err != nil {
		return 0, err
	}

	var alive []ManagedMarker
	if opts.All {
		alive, err = s.aliveSSHBlocks(ssh, stale, unverified)
		if err != nil {
			return 0, err
		}
	}

	if len(stale) == 0 && len(unverified) == 0 && len(alive) == 0 {
		s.Report.Success("No stale SSH config blocks found.")
		// Keys can outlive their block: destroy removes the block on its own, and
		// a container coming back on another address leaves the old one pinned.
		return 0, s.cleanKnownHosts(ssh, opts)
	}

	if len(stale) > 0 {
		s.Report.Info("Found %d stale SSH config block(s):", len(stale))
		for _, b := range stale {
			s.Report.Info("  - Host %s  (%s %s)%s", b.Alias, b.Kind, b.Ref, viaOriginNote(b))
		}
	}
	if len(unverified) > 0 {
		s.Report.Warn("Found %d SSH config block(s) whose remote host could not be reached (not removed automatically):", len(unverified))
		for _, b := range unverified {
			s.Report.Info("  - Host %s  (%s %s)%s", b.Alias, b.Kind, b.Ref, viaOriginNote(b))
		}
	}
	if len(alive) > 0 {
		s.Report.Info("Also listing %d SSH config block(s) that are currently alive (--all):", len(alive))
		for _, b := range alive {
			s.Report.Info("  - Host %s  (%s %s)%s", b.Alias, b.Kind, b.Ref, viaOriginNote(b))
		}
	}

	if opts.DryRun {
		s.Report.Warn("Dry run — nothing removed. Re-run without --dry-run to apply.")
		return 0, nil
	}

	var toRemove []ManagedMarker
	if opts.Yes {
		// --all only widens what is shown/selectable; --yes never auto-removes
		// an alive or unverified block just because it was listed.
		toRemove = stale
		if len(unverified) > 0 {
			s.Report.Warn("Skipping %d unverified block(s) in non-interactive mode; re-run interactively to remove them.", len(unverified))
		}
		if len(alive) > 0 {
			s.Report.Warn("Skipping %d alive block(s) in non-interactive mode; re-run interactively to remove them.", len(alive))
		}
	} else {
		if !opts.Interactive || s.Prompt == nil {
			return 0, fmt.Errorf("refusing to remove SSH config blocks without confirmation; pass --yes to confirm in non-interactive mode")
		}
		picked, err := s.pickSSHBlocksToRemove(stale, unverified, alive)
		if err != nil {
			return 0, err
		}
		if len(picked) == 0 {
			s.Report.Warn("Cancelled.")
			return 0, nil
		}
		toRemove = picked
	}

	if len(toRemove) == 0 {
		s.Report.Success("Nothing removed.")
		return 0, s.cleanKnownHosts(ssh, opts)
	}

	removed, backup, err := ssh.RemoveManagedBlocks(toRemove)
	if err != nil {
		return 0, err
	}
	if backup != "" {
		s.Report.Info("Backup saved: %s", backup)
	}
	s.Report.Success("Removed %d SSH config block(s) from the managed SSH config.", len(removed))

	// Runs after the blocks are gone, so the host keys they pinned are orphaned
	// by then and get swept in the same pass.
	if err := s.cleanKnownHosts(ssh, opts); err != nil {
		return len(removed), err
	}
	return len(removed), nil
}

// cleanKnownHosts drops the entries in the CLI-managed known_hosts that no Host
// block in either SSH config dials any more. It is keyed by address, not by marker,
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
	if !s.reportSelection("networks", anyExist, len(candidates)) {
		return 0, nil
	}

	s.Report.Warn("\nnetworks to remove (%d):", len(candidates))
	for _, n := range candidates {
		s.Report.Info("  %s", n.Name)
	}

	if opts.DryRun {
		s.dryRunNotice()
		return 0, nil
	}
	proceed, err := s.confirmDestructive(opts, fmt.Errorf("cannot remove networks in non-interactive mode without --yes"), "Remove these networks?")
	if err != nil {
		return 0, err
	}
	if !proceed {
		return 0, nil
	}

	removed, failed := s.RemoveNetworks(candidates)
	s.reportRemoval("networks", removed, failed)
	return removed, nil
}

// CleanVolumes removes CLI-managed Docker volumes.
func (s PruneService) CleanVolumes(opts CleanOptions) (int, error) {
	candidates, anyExist := s.SelectVolumes(opts.All)
	if !s.reportSelection("volumes", anyExist, len(candidates)) {
		return 0, nil
	}

	s.Report.Warn("\nvolumes to remove (%d):", len(candidates))
	for _, v := range candidates {
		s.Report.Info("  %s", v.Name)
	}

	if opts.DryRun {
		s.dryRunNotice()
		return 0, nil
	}
	proceed, err := s.confirmDestructive(opts, fmt.Errorf("cannot remove volumes in non-interactive mode without --yes"), "Remove these volumes?")
	if err != nil {
		return 0, err
	}
	if !proceed {
		return 0, nil
	}

	removed, failed := s.RemoveVolumes(candidates)
	s.reportRemoval("volumes", removed, failed)
	return removed, nil
}

type cleanTargets struct {
	catalog       []string
	containers    []LocalContainer
	images        []LocalImage
	ssh           []ManagedMarker
	networks      []LocalNetwork
	volumes       []LocalVolume
	anyContainers bool
	anyImages     bool
	anyNetworks   bool
	anyVolumes    bool
}

func (t cleanTargets) total() int {
	return len(t.catalog) + len(t.containers) + len(t.images) + len(t.ssh) + len(t.networks) + len(t.volumes)
}

func (t cleanTargets) anyManagedResource() bool {
	return t.anyContainers || t.anyImages || t.anyNetworks || t.anyVolumes ||
		len(t.catalog) > 0 || len(t.ssh) > 0
}

// CleanAll sweeps every category (catalog, containers, images, ssh, networks, volumes).
func (s PruneService) CleanAll(opts CleanOptions) error {
	targets := s.collectCleanTargets(opts)

	if !targets.anyManagedResource() {
		s.Report.Info("No devcontainer resources or stale entries found locally.")
		return nil
	}
	if targets.total() == 0 {
		s.Report.Info("No unused devcontainer resources or stale entries found.")
		s.Report.Warn("Use --all to remove every managed resource.")
		return nil
	}

	s.previewCleanTargets(targets)

	if opts.DryRun {
		s.dryRunNotice()
		return nil
	}
	proceed, err := s.confirmDestructive(opts, fmt.Errorf("cannot clean resources in non-interactive mode without --yes"), "Remove these resources and stale entries?")
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	removedTotal, failedTotal := s.removeCleanTargets(targets)
	s.reportRemoval("item(s)", removedTotal, failedTotal)
	return nil
}

func (s PruneService) collectCleanTargets(opts CleanOptions) cleanTargets {
	sshSvc := SshService{Report: s.Report}

	targets := cleanTargets{}
	for _, e := range domain.ListEntries() {
		if _, statErr := os.Stat(e.ProjectDir); os.IsNotExist(statErr) {
			targets.catalog = append(targets.catalog, e.ProjectDir)
		}
	}
	targets.containers, targets.anyContainers = s.SelectContainers(opts.All)
	targets.images, targets.anyImages = s.SelectImages(opts.All)
	if existsWs, existsCnt, existsRemoteCnt, err := sshSvc.LiveTargetPredicates(); err == nil {
		// No per-item picker here, so unverified --via blocks are left out —
		// run 'clean ssh' directly to review/remove those.
		targets.ssh, _, _, _ = sshSvc.PruneManagedBlocks(existsWs, existsCnt, existsRemoteCnt, true)
	}
	targets.networks, targets.anyNetworks = s.SelectNetworks(opts.All)
	targets.volumes, targets.anyVolumes = s.SelectVolumes(opts.All)
	return targets
}

func (s PruneService) previewCleanTargets(t cleanTargets) {
	if len(t.catalog) > 0 {
		s.Report.Warn("\nStale catalog entries to remove (%d):", len(t.catalog))
		for _, dir := range t.catalog {
			s.Report.Info("  %s", dir)
		}
	}
	if len(t.containers) > 0 {
		s.Report.Warn("\nContainers to remove (%d):", len(t.containers))
		for _, c := range t.containers {
			s.Report.Info("  %s", containerLabel(c))
		}
	}
	if len(t.images) > 0 {
		s.Report.Warn("\nImages to remove (%d):", len(t.images))
		for _, img := range t.images {
			s.Report.Info("  %s  (%s)", img.Ref, img.ID)
		}
	}
	if len(t.ssh) > 0 {
		s.Report.Warn("\nStale SSH config blocks to remove (%d):", len(t.ssh))
		for _, b := range t.ssh {
			s.Report.Info("  Host %s  (%s %s)%s", b.Alias, b.Kind, b.Ref, viaOriginNote(b))
		}
	}
	if len(t.networks) > 0 {
		s.Report.Warn("\nNetworks to remove (%d):", len(t.networks))
		for _, net := range t.networks {
			s.Report.Info("  %s", net.Name)
		}
	}
	if len(t.volumes) > 0 {
		s.Report.Warn("\nVolumes to remove (%d):", len(t.volumes))
		for _, vol := range t.volumes {
			s.Report.Info("  %s", vol.Name)
		}
	}
}

func (s PruneService) removeCleanTargets(t cleanTargets) (removedTotal, failedTotal int) {
	sshSvc := SshService{Report: s.Report}

	for _, dir := range t.catalog {
		if domain.RemoveEntry(dir) {
			removedTotal++
		}
	}
	if len(t.containers) > 0 {
		removed, failed := s.RemoveContainers(t.containers)
		removedTotal += removed
		failedTotal += failed
	}
	if len(t.images) > 0 {
		removed, failed := s.Remove(t.images)
		removedTotal += removed
		failedTotal += failed
	}
	if len(t.ssh) > 0 {
		if res, backup, err := sshSvc.RemoveManagedBlocks(t.ssh); err == nil {
			removedTotal += len(res)
			if backup != "" {
				s.Report.Info("SSH backup saved: %s", backup)
			}
		}
	}
	if len(t.networks) > 0 {
		removed, failed := s.RemoveNetworks(t.networks)
		removedTotal += removed
		failedTotal += failed
	}
	if len(t.volumes) > 0 {
		removed, failed := s.RemoveVolumes(t.volumes)
		removedTotal += removed
		failedTotal += failed
	}
	return removedTotal, failedTotal
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
		{Value: "ssh", Label: "Stale SSH config blocks (managed SSH config)"},
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
	var out []LocalImage
	for _, line := range captureLines([]string{
		"images", "--filter", managedFilter,
		"--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}",
	}) {
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
	return removeResources(images,
		func(i LocalImage) []string { return []string{"rmi", i.Ref} },
		func(i LocalImage) {
			s.Report.Error("  ✗ Failed to remove %s (container may be running)", i.Ref)
		})
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
	var out []LocalContainer
	for _, line := range captureLines([]string{
		"ps", "-a", "--filter", managedFilter,
		"--format", "{{.Names}}\t{{.State}}",
	}) {
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
	return removeResources(containers,
		func(c LocalContainer) []string { return []string{"rm", "-f", c.Name} },
		func(c LocalContainer) { s.Report.Error("  ✗ Failed to remove container %s", c.Name) })
}

// ---------------------------------------------------------------------------
// Networks
// ---------------------------------------------------------------------------

// LocalNetwork is one network created by this CLI.
type LocalNetwork struct {
	Name string
}

func listCliNetworks() []LocalNetwork {
	var out []LocalNetwork
	for _, name := range captureLines([]string{
		"network", "ls", "--filter", managedFilter, "--format", "{{.Name}}",
	}) {
		out = append(out, LocalNetwork{Name: name})
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
	return removeResources(networks,
		func(n LocalNetwork) []string { return []string{"network", "rm", n.Name} },
		func(n LocalNetwork) { s.Report.Error("  ✗ Failed to remove network %s", n.Name) })
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
	var out []LocalVolume
	for _, name := range captureLines(args) {
		out = append(out, LocalVolume{Name: name})
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
	return removeResources(volumes,
		func(v LocalVolume) []string { return []string{"volume", "rm", v.Name} },
		func(v LocalVolume) { s.Report.Error("  ✗ Failed to remove volume %s", v.Name) })
}
