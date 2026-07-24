package service

import (
	"path/filepath"
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
	runner := &fakeRunner{status: 0, stdout: "devcontainer-shared-config\nmyproj_devcontainer_etc"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	volumes, anyExist := svc.SelectVolumes(true)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	for _, v := range volumes {
		if v.Name == "devcontainer-shared-config" {
			t.Errorf("shared-config volume must be excluded by default, got %v", volumes)
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
