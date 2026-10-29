package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

func TestImageService_CreateListInfoRemove(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &fakeRunner{}
	docker.SetRunner(runner)
	t.Cleanup(docker.ResetRunner)

	svc := ImageService{
		Report:    nopReporter{},
		ImagesDir: tmpDir,
	}

	// 1. Create from profile
	err := svc.Create("test-node", CreateImageOpts{
		FromProfile: "nodejs",
		Tag:         "v1.0",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 2. List
	statuses, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "test-node" {
		t.Fatalf("expected 1 image status named test-node, got %v", statuses)
	}
	if statuses[0].FullTag != "devcontainer-cli-img/test-node:v1.0" {
		t.Errorf("unexpected FullTag %s", statuses[0].FullTag)
	}

	// 3. Info
	info, err := svc.Info("test-node")
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	if info.Tag != "v1.0" {
		t.Errorf("expected tag v1.0, got %s", info.Tag)
	}

	// 4. Export
	exportDir := filepath.Join(tmpDir, "export-out")
	if err := svc.Export("test-node", exportDir); err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportDir, "Dockerfile")); err != nil {
		t.Errorf("expected Dockerfile in export dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportDir, "BUILD.md")); err != nil {
		t.Errorf("expected BUILD.md in export dir: %v", err)
	}

	// 5. Remove
	if err := svc.Remove("test-node", true); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	statusesAfter, _ := svc.List()
	if len(statusesAfter) != 0 {
		t.Errorf("expected 0 images after remove, got %d", len(statusesAfter))
	}
}
