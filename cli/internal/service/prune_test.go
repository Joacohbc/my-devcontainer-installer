package service

import (
	"os"
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

func TestPruneSelectNoneWhenNoImages(t *testing.T) {
	runner := &fakeRunner{status: 0, stdout: ""}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if images, anyExist := svc.SelectImages(false); anyExist || images != nil {
		t.Errorf("expected no images; got %v anyExist=%v", images, anyExist)
	}
}

func TestPruneSelectOrphansOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	liveDir := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	domain.RecordProject(liveDir, &types.DevcontainerConfig{Workspace: "ws"}, "devcontainer-cli/abc:latest")

	runner := &fakeRunner{status: 0, stdout: "devcontainer-cli/abc:latest\tID1\ndevcontainer-cli/orphan:latest\tID2"}
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	toRemove, anyExist := svc.SelectImages(false)
	if !anyExist {
		t.Fatal("expected anyExist=true")
	}
	if len(toRemove) != 1 || toRemove[0].Ref != "devcontainer-cli/orphan:latest" {
		t.Errorf("expected only the orphan image, got %v", toRemove)
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
	restore()

	failRunner := &fakeRunner{status: 1}
	defer useFakeDocker(failRunner)()
	if removed, failed := svc.Remove(images); removed != 0 || failed != 2 {
		t.Errorf("fail runner: removed=%d failed=%d, want 0/2", removed, failed)
	}
}

func TestPruneSelectAllParsesNetworksAndVolumes(t *testing.T) {
	netRunner := &fakeRunner{status: 0, stdout: "net1\tproj1\nnet2\tproj2"}
	restore := useFakeDocker(netRunner)

	svc := PruneService{Report: nopReporter{}}
	networks, anyExist := svc.SelectNetworks(true)
	if !anyExist {
		t.Fatal("expected anyExist=true for networks")
	}
	if len(networks) != 2 {
		t.Fatalf("got %d networks, want 2: %v", len(networks), networks)
	}
	if networks[0].Name != "net1" || networks[0].Project != "proj1" {
		t.Errorf("unexpected first network: %+v", networks[0])
	}
	restore()

	volRunner := &fakeRunner{status: 0, stdout: "vol1\tproj1\nvol2\tproj2"}
	restore = useFakeDocker(volRunner)
	volumes, anyExist := svc.SelectVolumes(true)
	if !anyExist {
		t.Fatal("expected anyExist=true for volumes")
	}
	if len(volumes) != 2 {
		t.Fatalf("got %d volumes, want 2: %v", len(volumes), volumes)
	}
	if volumes[0].Name != "vol1" || volumes[0].Project != "proj1" {
		t.Errorf("unexpected first volume: %+v", volumes[0])
	}
	restore()
}

func TestPruneSelectOrphanNetworksAndVolumes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	liveDir := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	domain.RecordProject(liveDir, &types.DevcontainerConfig{Workspace: "ws"}, "devcontainer-cli/abc:latest")

	netRunner := &fakeRunner{status: 0, stdout: "net_live\tdevcontainer-cli_abc_latest\nnet_orphan\tdevcontainer-cli_orphan_latest\nnet_no_proj\t"}
	restore := useFakeDocker(netRunner)

	svc := PruneService{Report: nopReporter{}}
	toRemoveNets, anyNets := svc.SelectNetworks(false)
	if !anyNets {
		t.Fatal("expected anyNets=true")
	}
	if len(toRemoveNets) != 2 {
		t.Fatalf("expected 2 orphan networks, got %v", toRemoveNets)
	}
	restore()

	volRunner := &fakeRunner{status: 0, stdout: "vol_live\tdevcontainer-cli_abc_latest\nvol_orphan\tdevcontainer-cli_orphan_latest\nvol_no_proj\t"}
	restore = useFakeDocker(volRunner)
	toRemoveVols, anyVols := svc.SelectVolumes(false)
	if !anyVols {
		t.Fatal("expected anyVols=true")
	}
	if len(toRemoveVols) != 2 {
		t.Fatalf("expected 2 orphan volumes, got %v", toRemoveVols)
	}
	restore()
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
