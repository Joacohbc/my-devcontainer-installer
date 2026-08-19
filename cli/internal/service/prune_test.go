package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestPruneSelectAllParsesImages(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "devcontainer-cli/abc:latest\tID1\ndevcontainer-cli/def:latest\tID2"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	images, anyExist := svc.SelectImages(true)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2: %v", len(images), images)
	}
	if images[0].Ref != "devcontainer-cli/abc:latest" || images[0].ID != "ID1" {
		t.Errorf("unexpected first image: %+v", images[0])
	}
}

func TestPruneSelectAllParsesRemoteImages(t *testing.T) {
	// Remote-mode images (ghcr.io/<owner>/devcontainer-*) carry the managed
	// label, so the label-based filter must surface them just like local ones.
	runner := &fakeRunner{status: 0, stdout: "ghcr.io/joacohbc/devcontainer-nodejs:latest\tID1\nghcr.io/joacohbc/devcontainer-go:latest\tID2\nnode-python:local\tID3"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	images, anyExist := svc.SelectImages(true)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	if len(images) != 3 {
		t.Fatalf("got %d images, want 3: %v", len(images), images)
	}
	if images[0].Ref != "ghcr.io/joacohbc/devcontainer-nodejs:latest" || images[0].ID != "ID1" {
		t.Errorf("unexpected first remote image: %+v", images[0])
	}
}

func TestFilterUnusedImages(t *testing.T) {
	images := []LocalImage{
		{Ref: "devcontainer-cli/abc:latest", ID: "ID1"},
		{Ref: "devcontainer-cli/def:latest", ID: "ID2"},
		{Ref: "ghcr.io/joacohbc/devcontainer-go:latest", ID: "ID3"},
	}
	// abc is in use by ref, def by ID; go is unused.
	inUse := map[string]bool{"devcontainer-cli/abc:latest": true, "ID2": true}

	got := filterUnusedImages(images, inUse)
	if len(got) != 1 || got[0].Ref != "ghcr.io/joacohbc/devcontainer-go:latest" {
		t.Fatalf("expected only the unused go image, got %v", got)
	}
}

func TestPruneSelectNoneWhenNoImages(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if images, anyExist := svc.SelectImages(false); anyExist || images != nil {
		t.Errorf("expected no images; got %v anyExist=%v", images, anyExist)
	}
}

func TestPruneRemoveCounts(t *testing.T) {
	images := []LocalImage{{Ref: "devcontainer-cli/a:latest"}, {Ref: "devcontainer-cli/b:latest"}}

	okRunner := &fakeRunner{status: 0}
	restore := useFakeDocker(okRunner)
	svc := PruneService{Report: nopReporter{}}
	if removed, failed := svc.Remove(images); removed != 2 || failed != 0 {
		t.Errorf("ok runner: removed=%d failed=%d, want 2/0", removed, failed)
	}
	if call := okRunner.callContaining("rmi"); call == nil {
		t.Error("expected an rmi call")
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if removed, failed := svc.Remove(images); removed != 0 || failed != 2 {
		t.Errorf("fail runner: removed=%d failed=%d, want 0/2", removed, failed)
	}
}

func TestPruneSelectContainers(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "c1\trunning\nc2\texited\nc3\tcreated"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}

	all, anyExist := svc.SelectContainers(true)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	if len(all) != 3 {
		t.Fatalf("all=true: got %d containers, want 3: %v", len(all), all)
	}

	unused, _ := svc.SelectContainers(false)
	if len(unused) != 2 {
		t.Fatalf("all=false: got %d containers, want 2 (non-running): %v", len(unused), unused)
	}
	for _, c := range unused {
		if c.State == "running" {
			t.Errorf("running container %q should not be selected when all=false", c.Name)
		}
	}
}

func TestPruneSelectContainersNone(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if got, anyExist := svc.SelectContainers(false); anyExist || got != nil {
		t.Errorf("expected no containers; got %v anyExist=%v", got, anyExist)
	}
}

func TestPruneRemoveContainers(t *testing.T) {
	containers := []LocalContainer{{Name: "c1"}, {Name: "c2"}}

	okRunner := &fakeRunner{status: 0}
	restore := useFakeDocker(okRunner)
	svc := PruneService{Report: nopReporter{}}
	if removed, failed := svc.RemoveContainers(containers); removed != 2 || failed != 0 {
		t.Errorf("ok runner: removed=%d failed=%d, want 2/0", removed, failed)
	}
	if call := okRunner.callContaining("rm"); call == nil {
		t.Error("expected an rm call")
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if removed, failed := svc.RemoveContainers(containers); removed != 0 || failed != 2 {
		t.Errorf("fail runner: removed=%d failed=%d, want 0/2", removed, failed)
	}
}

func TestPruneSelectAllParsesNetworksAndVolumes(t *testing.T) {
	netRunner := &fakeRunner{status: 0, stdout: "net1\nnet2"}
	restore := useFakeDocker(netRunner)

	svc := PruneService{Report: nopReporter{}}
	networks, anyExist := svc.SelectNetworks(true)
	if !anyExist {
		t.Fatal("expected anyExist=true for networks")
	}
	if len(networks) != 2 {
		t.Fatalf("got %d networks, want 2: %v", len(networks), networks)
	}
	if networks[0].Name != "net1" {
		t.Errorf("unexpected first network: %+v", networks[0])
	}
	restore()

	volRunner := &fakeRunner{status: 0, stdout: "vol1\nvol2"}
	restore = useFakeDocker(volRunner)
	volumes, anyExist := svc.SelectVolumes(true)
	if !anyExist {
		t.Fatal("expected anyExist=true for volumes")
	}
	if len(volumes) != 2 {
		t.Fatalf("got %d volumes, want 2: %v", len(volumes), volumes)
	}
	if volumes[0].Name != "vol1" {
		t.Errorf("unexpected first volume: %+v", volumes[0])
	}
	restore()
}

func TestPruneSelectVolumesExcludesSharedConfigByDefault(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "devcontainer-shared-config\ndevcontainer-router-data\ndevcontainer-router-config\nmyproj_devcontainer_etc"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	volumes, anyExist := svc.SelectVolumes(true)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	for _, v := range volumes {
		if v.Name == types.SharedConfigVolumeName || v.Name == domain.RouterDataVolumeName || v.Name == domain.RouterConfigVolumeName {
			t.Errorf("global volume %s must be excluded by default, got %v", v.Name, volumes)
		}
	}
	if len(volumes) != 1 || volumes[0].Name != "myproj_devcontainer_etc" {
		t.Errorf("expected only the persistence volume, got %v", volumes)
	}
}

func TestPruneSelectVolumesIncludesSharedConfigWhenAsked(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "devcontainer-shared-config\nmyproj_devcontainer_etc"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}, IncludeSharedConfig: true}
	volumes, _ := svc.SelectVolumes(true)
	var found bool
	for _, v := range volumes {
		if v.Name == "devcontainer-shared-config" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected shared-config volume to be included, got %v", volumes)
	}
}

func TestFilterUnusedNetworks(t *testing.T) {
	networks := []LocalNetwork{{Name: "net_used"}, {Name: "net_free"}}
	inUse := map[string]bool{"net_used": true}

	got := filterUnusedNetworks(networks, inUse)
	if len(got) != 1 || got[0].Name != "net_free" {
		t.Fatalf("expected only net_free, got %v", got)
	}
}

func TestPruneSelectVolumesUnusedAppliesDanglingFilter(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: "vol1\nvol2"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if _, anyExist := svc.SelectVolumes(false); !anyExist {
		t.Fatal("expected anyExist=true")
	}
	if call := runner.callContaining("dangling=true"); call == nil {
		t.Error("expected the unused volume query to use the dangling=true filter")
	}
}

func TestPruneRemoveNetworksAndVolumes(t *testing.T) {
	networks := []LocalNetwork{{Name: "net1"}, {Name: "net2"}}
	volumes := []LocalVolume{{Name: "vol1"}, {Name: "vol2"}}

	okRunner := &fakeRunner{status: 0}
	restore := useFakeDocker(okRunner)
	svc := PruneService{Report: nopReporter{}}
	if removed, failed := svc.RemoveNetworks(networks); removed != 2 || failed != 0 {
		t.Errorf("ok runner networks: removed=%d failed=%d, want 2/0", removed, failed)
	}
	if removed, failed := svc.RemoveVolumes(volumes); removed != 2 || failed != 0 {
		t.Errorf("ok runner volumes: removed=%d failed=%d, want 2/0", removed, failed)
	}
	restore()

	failRunner := &fakeRunner{status: 1}
	restore = useFakeDocker(failRunner)
	if removed, failed := svc.RemoveNetworks(networks); removed != 0 || failed != 2 {
		t.Errorf("fail runner networks: removed=%d failed=%d, want 0/2", removed, failed)
	}
	if removed, failed := svc.RemoveVolumes(volumes); removed != 0 || failed != 2 {
		t.Errorf("fail runner volumes: removed=%d failed=%d, want 0/2", removed, failed)
	}
	restore()
}

func TestCleanCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	existingDir := t.TempDir()
	missingDir := filepath.Join(tmpDir, "nonexistent-project")

	domain.RecordProject(existingDir, &types.DevcontainerConfig{Workspace: "ws1"}, "")
	domain.RecordProject(missingDir, &types.DevcontainerConfig{Workspace: "ws2"}, "")

	svc := PruneService{Report: nopReporter{}}

	// Dry run test
	removedDry, err := svc.CleanCatalog(CleanOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if len(removedDry) != 1 || removedDry[0] != missingDir {
		t.Errorf("dry run removed = %v, want [%s]", removedDry, missingDir)
	}
	if entries := domain.ListEntries(); len(entries) != 2 {
		t.Errorf("dry run should not modify registry, got %d entries", len(entries))
	}

	// Non-dry run test
	removed, err := svc.CleanCatalog(CleanOptions{Yes: true})
	if err != nil {
		t.Fatalf("unexpected error on clean: %v", err)
	}
	if len(removed) != 1 || removed[0] != missingDir {
		t.Errorf("removed = %v, want [%s]", removed, missingDir)
	}
	entries := domain.ListEntries()
	if len(entries) != 1 || entries[0].ProjectDir != existingDir {
		t.Errorf("after clean catalog, remaining entries = %v, want only [%s]", entries, existingDir)
	}
}

func TestSelectByNames(t *testing.T) {
	images := []LocalImage{
		{Ref: "devcontainer-cli/a:latest", ID: "ID1"},
		{Ref: "devcontainer-cli/b:latest", ID: "ID2"},
		{Ref: "ghcr.io/o/devcontainer-go:latest", ID: "ID3"},
	}
	nameOf := func(i LocalImage) string { return i.Ref }

	selected, missing := selectByNames(images, []string{"devcontainer-cli/b:latest", "ghcr.io/o/devcontainer-go:latest"}, nameOf)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 2 || selected[0].Ref != "devcontainer-cli/b:latest" || selected[1].Ref != "ghcr.io/o/devcontainer-go:latest" {
		t.Fatalf("selected mismatch (order should follow names): %+v", selected)
	}

	selected, missing = selectByNames(images, []string{"devcontainer-cli/a:latest", "nope:latest"}, nameOf)
	if len(selected) != 1 || selected[0].Ref != "devcontainer-cli/a:latest" {
		t.Errorf("expected only the matching image, got %+v", selected)
	}
	if len(missing) != 1 || missing[0] != "nope:latest" {
		t.Errorf("expected missing [nope:latest], got %v", missing)
	}
}

// writeSSHFiles seeds ~/.ssh/config and the CLI-managed known_hosts for the
// clean-ssh tests, pointing both HOME and XDG_CONFIG_HOME at temp dirs.
func writeSSHFiles(t *testing.T, config, knownHosts string) (configPath, knownHostsPath string) {
	t.Helper()
	home := t.TempDir()
	setHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	configPath = managedSSHConfig(home)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	knownHostsPath = domain.ManagedKnownHostsPath()
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, knownHostsPath
}

// Both marker kinds are pruned the same way, and the host keys the removed
// blocks had pinned go with them.
func TestCleanSSHRemovesBlocksAndPinnedKeys(t *testing.T) {
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n" +
		"Host dc-ssh\n    HostName 172.25.2.30\n    User devuser\n\n" +
		"Host mine\n    HostName example.com\n"
	knownHosts := "172.25.1.30 ssh-ed25519 WS\n172.25.2.30 ssh-ed25519 LOOSE\nexample.com ssh-ed25519 KEEP\n"
	configPath, knownHostsPath := writeSSHFiles(t, config, knownHosts)

	// No managed containers and an empty registry: both targets read as gone.
	defer useFakeDocker(&fakeRunner{status: 0, stdout: ""})()

	svc := PruneService{Report: nopReporter{}}
	removed, err := svc.CleanSSH(CleanOptions{Yes: true})
	if err != nil {
		t.Fatalf("CleanSSH: %v", err)
	}
	if removed != 2 {
		t.Fatalf("CleanSSH removed %d blocks, want 2 (workspace + container)", removed)
	}

	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"Host api", "Host dc-ssh"} {
		if strings.Contains(string(gotConfig), gone) {
			t.Errorf("config still has %q:\n%s", gone, gotConfig)
		}
	}
	if !strings.Contains(string(gotConfig), "Host mine") {
		t.Errorf("clean ssh dropped a block it does not manage:\n%s", gotConfig)
	}

	gotKeys, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	// The hand-written block still dials example.com, so its key survives.
	if string(gotKeys) != "example.com ssh-ed25519 KEEP\n" {
		t.Errorf("known_hosts = %q, want only the still-referenced host", gotKeys)
	}
}

// destroy removes a block without touching known_hosts, so clean ssh sweeps the
// leftover key even when no block is stale.
func TestCleanSSHSweepsOrphanKeysWithoutStaleBlocks(t *testing.T) {
	_, knownHostsPath := writeSSHFiles(t,
		"Host mine\n    HostName example.com\n",
		"172.25.1.30 ssh-ed25519 DESTROYED\nexample.com ssh-ed25519 KEEP\n")
	defer useFakeDocker(&fakeRunner{status: 0, stdout: ""})()

	svc := PruneService{Report: nopReporter{}}
	removed, err := svc.CleanSSH(CleanOptions{Yes: true})
	if err != nil || removed != 0 {
		t.Fatalf("CleanSSH = %d,%v, want 0,nil", removed, err)
	}
	gotKeys, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotKeys) != "example.com ssh-ed25519 KEEP\n" {
		t.Errorf("known_hosts = %q, want the orphaned key swept", gotKeys)
	}
}

func TestCleanSSHDryRunKeepsPinnedKeys(t *testing.T) {
	knownHosts := "172.25.1.30 ssh-ed25519 WS\n"
	configPath, knownHostsPath := writeSSHFiles(t,
		"# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\nHost api\n    HostName 172.25.1.30\n",
		knownHosts)
	defer useFakeDocker(&fakeRunner{status: 0, stdout: ""})()

	svc := PruneService{Report: nopReporter{}}
	if _, err := svc.CleanSSH(CleanOptions{DryRun: true}); err != nil {
		t.Fatalf("CleanSSH dry run: %v", err)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotConfig), "Host api") {
		t.Errorf("dry run removed the block:\n%s", gotConfig)
	}
	gotKeys, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotKeys) != knownHosts {
		t.Errorf("dry run rewrote known_hosts: %q", gotKeys)
	}
}

// --all lists a currently-alive block alongside the stale one, but --yes must
// never auto-remove it: only the confirmed-stale block goes.
func TestCleanSSHAll_YesListsButDoesNotRemoveAliveBlocks(t *testing.T) {
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n" +
		"Host dc-ssh\n    HostName 172.25.2.30\n    User devuser\n"
	configPath, _ := writeSSHFiles(t, config, "")

	stdout := `{"Names":"api-devcontainer-ssh","Image":"img","Status":"Up","State":"running","Labels":"` +
		types.LabelManaged + `=true,com.docker.compose.project=api","Ports":""}`
	defer useFakeDocker(&fakeRunner{status: 0, stdout: stdout})()

	svc := PruneService{Report: nopReporter{}}
	removed, err := svc.CleanSSH(CleanOptions{Yes: true, All: true})
	if err != nil {
		t.Fatalf("CleanSSH: %v", err)
	}
	if removed != 1 {
		t.Fatalf("CleanSSH removed %d, want 1 (only the stale container block)", removed)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotConfig), "Host api") {
		t.Errorf("--all + --yes must not auto-remove an alive block:\n%s", gotConfig)
	}
	if strings.Contains(string(gotConfig), "Host dc-ssh") {
		t.Errorf("the stale block should still be removed:\n%s", gotConfig)
	}
}

// Without --all, an alive block is invisible to clean-ssh entirely — nothing
// stale, so it's a no-op.
func TestCleanSSHWithoutAll_IgnoresAliveBlocks(t *testing.T) {
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n"
	writeSSHFiles(t, config, "")

	stdout := `{"Names":"api-devcontainer-ssh","Image":"img","Status":"Up","State":"running","Labels":"` +
		types.LabelManaged + `=true,com.docker.compose.project=api","Ports":""}`
	defer useFakeDocker(&fakeRunner{status: 0, stdout: stdout})()

	svc := PruneService{Report: nopReporter{}}
	removed, err := svc.CleanSSH(CleanOptions{Yes: true})
	if err != nil || removed != 0 {
		t.Fatalf("CleanSSH = %d,%v, want 0,nil", removed, err)
	}
}

type multiselectingPrompter struct {
	scriptedPrompter
	gotChoices []Option
	gotInitial []Option
	pick       func([]Option) []Option
}

func (p *multiselectingPrompter) Multiselect(_ string, choices []Option, initial []Option) ([]Option, error) {
	p.gotChoices = choices
	p.gotInitial = initial
	if p.pick != nil {
		return p.pick(choices), nil
	}
	return initial, nil
}

// --all's alive block is offered in the interactive picker, but unchecked by
// default like an unverified one — only stale blocks come pre-selected.
func TestCleanSSHAll_Interactive_AliveBlockOfferedButNotPreselected(t *testing.T) {
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n" +
		"Host dc-ssh\n    HostName 172.25.2.30\n    User devuser\n"
	configPath, _ := writeSSHFiles(t, config, "")

	stdout := `{"Names":"api-devcontainer-ssh","Image":"img","Status":"Up","State":"running","Labels":"` +
		types.LabelManaged + `=true,com.docker.compose.project=api","Ports":""}`
	defer useFakeDocker(&fakeRunner{status: 0, stdout: stdout})()

	prompter := &multiselectingPrompter{}
	svc := PruneService{Report: nopReporter{}, Prompt: prompter}
	removed, err := svc.CleanSSH(CleanOptions{All: true, Interactive: true})
	if err != nil {
		t.Fatalf("CleanSSH: %v", err)
	}
	if len(prompter.gotChoices) != 2 {
		t.Fatalf("Multiselect choices = %d, want 2 (stale + alive)", len(prompter.gotChoices))
	}
	if len(prompter.gotInitial) != 1 {
		t.Errorf("Multiselect initial = %d, want 1 (only the stale block pre-checked)", len(prompter.gotInitial))
	}
	aliveLabel := prompter.gotChoices[1].Label
	if !strings.Contains(aliveLabel, "[ALIVE]") {
		t.Errorf("alive choice label = %q, want it marked [ALIVE]", aliveLabel)
	}
	// The default pick (== initial) removes only the stale block.
	if removed != 1 {
		t.Fatalf("CleanSSH removed %d, want 1", removed)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotConfig), "Host api") {
		t.Errorf("the alive block was not selected and must survive:\n%s", gotConfig)
	}
}

// Explicitly picking the alive block in the interactive picker does remove it
// — --all only changes what's offered, never what the user is allowed to do.
func TestCleanSSHAll_Interactive_CanExplicitlyRemoveAliveBlock(t *testing.T) {
	config := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n"
	configPath, _ := writeSSHFiles(t, config, "")

	stdout := `{"Names":"api-devcontainer-ssh","Image":"img","Status":"Up","State":"running","Labels":"` +
		types.LabelManaged + `=true,com.docker.compose.project=api","Ports":""}`
	defer useFakeDocker(&fakeRunner{status: 0, stdout: stdout})()

	prompter := &multiselectingPrompter{pick: func(choices []Option) []Option { return choices }}
	svc := PruneService{Report: nopReporter{}, Prompt: prompter}
	removed, err := svc.CleanSSH(CleanOptions{All: true, Interactive: true})
	if err != nil {
		t.Fatalf("CleanSSH: %v", err)
	}
	if removed != 1 {
		t.Fatalf("CleanSSH removed %d, want 1 (the explicitly-picked alive block)", removed)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotConfig), "Host api") {
		t.Errorf("explicitly picking the alive block should remove it:\n%s", gotConfig)
	}
}

type confirmingPrompter struct {
	scriptedPrompter
	answer bool
	err    error
	asked  bool
}

func (p *confirmingPrompter) Confirm(string) (bool, error) {
	p.asked = true
	return p.answer, p.err
}

func TestConfirmDestructive(t *testing.T) {
	niErr := errors.New("needs --yes")

	t.Run("--yes proceeds without prompting", func(t *testing.T) {
		p := &confirmingPrompter{answer: false}
		svc := PruneService{Report: nopReporter{}, Prompt: p}
		ok, err := svc.confirmDestructive(CleanOptions{Yes: true}, niErr, "Remove?")
		if err != nil || !ok {
			t.Fatalf("got ok=%v err=%v, want true/nil", ok, err)
		}
		if p.asked {
			t.Error("should not prompt when --yes is set")
		}
	})

	t.Run("non-interactive without --yes returns the error", func(t *testing.T) {
		svc := PruneService{Report: nopReporter{}}
		ok, err := svc.confirmDestructive(CleanOptions{Interactive: false}, niErr, "Remove?")
		if ok || err != niErr {
			t.Fatalf("got ok=%v err=%v, want false/%v", ok, err, niErr)
		}
	})

	t.Run("interactive confirm yes proceeds", func(t *testing.T) {
		p := &confirmingPrompter{answer: true}
		svc := PruneService{Report: nopReporter{}, Prompt: p}
		ok, err := svc.confirmDestructive(CleanOptions{Interactive: true}, niErr, "Remove?")
		if err != nil || !ok || !p.asked {
			t.Fatalf("got ok=%v err=%v asked=%v, want true/nil/true", ok, err, p.asked)
		}
	})

	t.Run("interactive confirm no cancels", func(t *testing.T) {
		p := &confirmingPrompter{answer: false}
		svc := PruneService{Report: nopReporter{}, Prompt: p}
		ok, err := svc.confirmDestructive(CleanOptions{Interactive: true}, niErr, "Remove?")
		if err != nil || ok {
			t.Fatalf("got ok=%v err=%v, want false/nil", ok, err)
		}
	})
}
