package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

const (
	projectImgA = "devcontainer-cli/go-11111111:latest\tIDA"
	projectImgB = "devcontainer-cli/custom-22222222:latest\tIDB"
	namedImg    = "devcontainer-cli-img/android:v1\tIDN"
	legacyNamed = "devcontainer-cli-img/old:latest\tIDL"
	danglingImg = "<none>:<none>\tIDD"
)

func rmiTargets(r *imageListRunner) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, call := range r.calls {
		if len(call) >= 3 && call[1] == "rmi" {
			out = append(out, call[2])
		}
	}
	slices.Sort(out)
	return out
}

func gcRunner(images ...string) *imageListRunner {
	return &imageListRunner{
		images: strings.Join(images, "\n"),
		named:  namedImg,
	}
}

// Named images are reusable definitions: they are identified by the kind label
// or, for images built before labels, by their namespace — and never selected
// by a bulk clean.
func TestSelectImagesNeverSelectsNamedImages(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defer useFakeDocker(gcRunner(projectImgA, namedImg, legacyNamed))()

	svc := PruneService{Report: nopReporter{}}
	selected, _ := svc.SelectImages(true)
	if len(selected) != 1 || selected[0].ID != "IDA" {
		t.Errorf("expected only the project image, got %+v", selected)
	}
	named := svc.SelectNamedImages(true)
	if len(named) != 2 {
		t.Errorf("expected both named images (label and legacy namespace), got %+v", named)
	}
}

// H5: a non-interactive clean (even with --yes) must keep named images.
func TestCleanImagesKeepsNamedImagesWithoutAnExplicitYes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := gcRunner(projectImgA, namedImg)
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if _, err := svc.CleanImages(nil, CleanOptions{All: true, Yes: true}); err != nil {
		t.Fatal(err)
	}
	if got := rmiTargets(runner); !slices.Equal(got, []string{"devcontainer-cli/go-11111111:latest"}) {
		t.Errorf("expected only the project image removed, got %v", got)
	}
}

func TestCleanImagesAsksBeforeRemovingNamedImages(t *testing.T) {
	for _, answer := range []bool{true, false} {
		t.Run(map[bool]string{true: "yes", false: "no"}[answer], func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			runner := gcRunner(projectImgA, namedImg)
			defer useFakeDocker(runner)()

			prompt := &confirmingPrompter{answer: answer}
			svc := PruneService{Report: nopReporter{}, Prompt: prompt}
			if _, err := svc.CleanImages(nil, CleanOptions{All: true, Yes: true, Interactive: true}); err != nil {
				t.Fatal(err)
			}
			if !prompt.asked {
				t.Fatal("expected a separate confirmation for named images")
			}
			want := []string{"devcontainer-cli/go-11111111:latest"}
			if answer {
				want = append(want, "devcontainer-cli-img/android:v1")
				slices.Sort(want)
			}
			if got := rmiTargets(runner); !slices.Equal(got, want) {
				t.Errorf("rmi targets = %v, want %v", got, want)
			}
		})
	}
}

// An explicit ref is an explicit decision, named image included.
func TestCleanImagesRemovesANamedImageNamedExplicitly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := gcRunner(projectImgA, namedImg)
	defer useFakeDocker(runner)()

	svc := PruneService{Report: nopReporter{}}
	if _, err := svc.CleanImages([]string{"devcontainer-cli-img/android:v1"}, CleanOptions{Yes: true}); err != nil {
		t.Fatal(err)
	}
	if got := rmiTargets(runner); !slices.Equal(got, []string{"devcontainer-cli-img/android:v1"}) {
		t.Errorf("got %v", got)
	}
}

// A rebuild moves the tag and leaves the old image as <none>:<none>, which
// docker cannot resolve by name: it must be removed by ID.
func TestDanglingImagesAreRemovedByID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := gcRunner(danglingImg)
	defer useFakeDocker(runner)()

	if n := (PruneService{Report: nopReporter{}}).CollectProjectImages(); n != 1 {
		t.Errorf("expected 1 removal, got %d", n)
	}
	if got := rmiTargets(runner); !slices.Equal(got, []string{"IDD"}) {
		t.Errorf("expected removal by ID, got %v", got)
	}
}

// Content-addressed images are reference counted: an image stays while any
// registered project or any container uses it, and goes once nothing does.
func TestCollectProjectImagesKeepsOnlyReferencedImages(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := domain.RecordEntry(domain.ImageEntry{ProjectDir: "/p/one", Image: "devcontainer-cli/go-11111111:latest"}); err != nil {
		t.Fatal(err)
	}
	runner := gcRunner(projectImgA, projectImgB, danglingImg, namedImg, "devcontainer-cli/run-33333333:latest\tIDC", "ghcr.io/joacohbc/devcontainer-nodejs:latest\tIDP")
	runner.containers = "devcontainer-cli/run-33333333:latest"
	defer useFakeDocker(runner)()

	(PruneService{Report: nopReporter{}}).CollectProjectImages()

	want := []string{"IDD", "devcontainer-cli/custom-22222222:latest"}
	if got := rmiTargets(runner); !slices.Equal(got, want) {
		t.Errorf("rmi targets = %v, want %v (registered, container-used, pulled and named images must stay)", got, want)
	}
}

// H1: destroying a project collects its image when no other project shares it.
func TestDestroyCollectsTheProjectImageUnlessShared(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "alone", true: "shared"}[shared], func(t *testing.T) {
			target := agentCleanProject(t)
			const image = "devcontainer-cli/go-11111111:latest"
			if err := domain.RecordEntry(domain.ImageEntry{ProjectDir: target.ProjectKey, Image: image}); err != nil {
				t.Fatal(err)
			}
			if shared {
				if err := domain.RecordEntry(domain.ImageEntry{ProjectDir: "/other/project", Image: image}); err != nil {
					t.Fatal(err)
				}
			}
			runner := gcRunner(projectImgA)
			defer useFakeDocker(runner)()

			if err := (DestroyService{Report: nopReporter{}}).Run(target); err != nil {
				t.Fatal(err)
			}
			got := rmiTargets(runner)
			if shared && len(got) != 0 {
				t.Errorf("a shared image must stay, got rmi %v", got)
			}
			if !shared && !slices.Equal(got, []string{image}) {
				t.Errorf("expected %s removed, got %v", image, got)
			}
		})
	}
}
