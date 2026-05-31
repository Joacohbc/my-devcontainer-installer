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
