package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
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

func TestImageServiceRecordsDefinitionChanges(t *testing.T) {
	docker.SetRunner(&fakeRunner{})
	t.Cleanup(docker.ResetRunner)

	vcs := &fakeVCS{root: true}
	repo, _ := newTestRepo(t, vcs)
	svc := ImageService{Report: nopReporter{}, ImagesDir: filepath.Join(repo.Dir, "images"), Repo: &repo}

	if err := svc.Create("android", CreateImageOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Create("android", CreateImageOpts{Tag: "v2"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove("android", false); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"feat(image): add android | images/android",
		"feat(image): update android | images/android",
		"chore(image): remove android | images/android",
	}
	if strings.Join(vcs.commits, "\n") != strings.Join(want, "\n") {
		t.Errorf("unexpected commits:\n%s", strings.Join(vcs.commits, "\n"))
	}
}

// A custom ImagesDir outside the configuration repository is not versioned.
func TestImageServiceCustomDirIsNotVersioned(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ImageService{Report: nopReporter{}, ImagesDir: t.TempDir()}
	if err := svc.Create("plain", CreateImageOpts{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "devcontainer-cli", ".git")); !os.IsNotExist(err) {
		t.Errorf("expected no configuration repository to be created, got %v", err)
	}
}

func createTestImageDef(t *testing.T, dir string) ImageService {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ImageService{Report: nopReporter{}, ImagesDir: dir}
	if err := svc.Create("android", CreateImageOpts{Tag: "v1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return svc
}

// H3: removing the Docker image of a named image some project still builds on
// must refuse before touching anything.
func TestImageRemoveRefusesWhileAProjectUsesTheImage(t *testing.T) {
	dir := t.TempDir()
	svc := createTestImageDef(t, dir)
	if err := domain.RecordEntry(domain.ImageEntry{ProjectDir: "/p/app", Image: "devcontainer-cli-img/android:v1"}); err != nil {
		t.Fatal(err)
	}
	runner := &imageListRunner{images: "IDN"}
	defer useFakeDocker(runner)()

	err := svc.Remove("android", true)
	if err == nil || !strings.Contains(err.Error(), "/p/app") {
		t.Fatalf("expected a refusal naming the project, got %v", err)
	}
	if runner.callContaining("rmi") != nil {
		t.Error("no image may be removed while in use")
	}
	if _, ok, _ := domain.ResolveImageDef("android", dir); !ok {
		t.Error("the definition must survive a refused removal")
	}
}

// H4: a failed rmi is reported, not swallowed.
func TestImageRemoveReportsAFailedRmi(t *testing.T) {
	svc := createTestImageDef(t, t.TempDir())
	defer useFakeDocker(&rmiFailingRunner{imageListRunner{images: "IDN"}})()

	if err := svc.Remove("android", true); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected the rmi failure to surface, got %v", err)
	}
}

type rmiFailingRunner struct{ imageListRunner }

func (r *rmiFailingRunner) Run(ctx context.Context, args []string, a, b string, env map[string]string) (int, string, string) {
	if len(args) >= 2 && args[1] == "rmi" {
		return 1, "", "conflict: image is being used"
	}
	return r.imageListRunner.Run(ctx, args, a, b, env)
}

func TestImageDefConfigIsLabelledNamed(t *testing.T) {
	config := domain.ImageDefConfig(domain.ImageDef{Name: "android", Tag: "v1"})
	config.Fingerprint = "feedfacecafe"
	if config.ImageKind != types.ImageKindNamed || config.Origin != "android" {
		t.Errorf("got kind=%q origin=%q", config.ImageKind, config.Origin)
	}
	if got := domain.ResolveDevcontainerImageName(config); got != "devcontainer-cli-img/android:v1" {
		t.Errorf("a named image keeps its own tag, got %q", got)
	}
}

// H3 for derivation: an image other definitions derive from is still in use.
func TestImageRemoveRefusesWhileAnImageDerivesFromIt(t *testing.T) {
	dir := t.TempDir()
	svc := createTestImageDef(t, dir)
	if err := svc.Create("child", CreateImageOpts{From: "android", Modules: []string{"sqlite"}}); err != nil {
		t.Fatal(err)
	}
	defer useFakeDocker(&imageListRunner{images: "IDN"})()

	err := svc.Remove("android", true)
	if err == nil || !strings.Contains(err.Error(), "image child") {
		t.Fatalf("expected a refusal naming the derived image, got %v", err)
	}
}

func TestImageNamesFiltersBySourceWithoutDocker(t *testing.T) {
	runner := &fakeRunner{}
	docker.SetRunner(runner)
	t.Cleanup(docker.ResetRunner)

	dir := t.TempDir()
	for _, def := range []domain.ImageDef{
		{Name: "web", Source: domain.ImageSourceBuild},
		{Name: "android", Source: domain.ImageSourceBuild},
		{Name: "legacy"},
		{Name: "team-base", Source: domain.ImageSourceRemote, Ref: "ghcr.io/acme/base:1"},
	} {
		if err := domain.SaveImageDef(def, dir); err != nil {
			t.Fatalf("SaveImageDef(%s): %v", def.Name, err)
		}
	}
	svc := ImageService{Report: nopReporter{}, ImagesDir: dir}

	cases := []struct {
		name    string
		sources []domain.ImageSource
		want    []string
	}{
		{"all", nil, []string{"android", "legacy", "team-base", "web"}},
		{"build only", []domain.ImageSource{domain.ImageSourceBuild}, []string{"android", "legacy", "web"}},
		{"remote only", []domain.ImageSource{domain.ImageSourceRemote}, []string{"team-base"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := svc.Names(c.sources...)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("Names(%v) = %v, want %v", c.sources, got, c.want)
			}
		})
	}
	if len(runner.calls) != 0 {
		t.Errorf("Names must not call docker, got %v", runner.calls)
	}
}

func TestImageNamesIsEmptyWithoutAReadableImagesDir(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{
		"missing":    filepath.Join(t.TempDir(), "missing"),
		"unreadable": notADir,
	} {
		t.Run(name, func(t *testing.T) {
			if got := (ImageService{Report: nopReporter{}, ImagesDir: dir}).Names(); len(got) != 0 {
				t.Errorf("Names() = %v, want none", got)
			}
		})
	}
}

func TestImagePullRefusesBuildDefinitions(t *testing.T) {
	for _, source := range []domain.ImageSource{domain.ImageSourceBuild, ""} {
		t.Run(string(source)+"-source", func(t *testing.T) {
			dir := t.TempDir()
			if err := domain.SaveImageDef(domain.ImageDef{Name: "web", Source: source}, dir); err != nil {
				t.Fatal(err)
			}
			runner := &fakeRunner{}
			docker.SetRunner(runner)
			t.Cleanup(docker.ResetRunner)

			err := ImageService{Report: nopReporter{}, ImagesDir: dir}.Pull("web")
			if err == nil || !strings.Contains(err.Error(), "devcontainer-cli image build web") {
				t.Fatalf("expected a refusal naming 'devcontainer-cli image build web', got %v", err)
			}
			if call := runner.callContaining("pull"); call != nil {
				t.Errorf("pull must not reach docker for a build definition, got %v", call)
			}
		})
	}
}
