package domain_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var (
	rootlessPodman = types.EngineTraits{IsPodman: true, IsRootless: true, SocketPath: "/run/user/1000/podman/podman.sock"}
	rootfulPodman  = types.EngineTraits{IsPodman: true, SocketPath: "/run/podman/podman.sock"}
)

func withEngine(traits types.EngineTraits) func(*types.DevcontainerConfig) {
	return func(c *types.DevcontainerConfig) { c.Engine = traits }
}

func withPostgresAndDod(c *types.DevcontainerConfig) {
	c.Dockerfile.Modules = []types.SelectedModule{{ID: "dod"}}
	c.Compose.Services = []types.SelectedService{{ID: types.ServiceID("postgres")}}
}

func composeServices(t *testing.T, cfg *types.DevcontainerConfig) map[string]map[string]any {
	t.Helper()
	var parsed struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(mustGenerateCompose(t, cfg)), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	return parsed.Services
}

func stringList(t *testing.T, service map[string]any, key string) []string {
	t.Helper()
	raw, _ := service[key].([]any)
	list := make([]string, len(raw))
	for i, item := range raw {
		list[i], _ = item.(string)
	}
	return list
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestGenerateCompose_DockerEngineKeepsTheDockerShape(t *testing.T) {
	services := composeServices(t, makeConfig(withPostgresAndDod))
	devcontainer := services["devcontainer-ssh"]

	if _, set := devcontainer["userns_mode"]; set {
		t.Errorf("docker must not get userns_mode, got %v", devcontainer["userns_mode"])
	}
	if _, set := devcontainer["user"]; set {
		t.Errorf("docker must not override the user, got %v", devcontainer["user"])
	}
	if containsString(stringList(t, devcontainer, "ports"), types.SSHLoopbackPublishing) {
		t.Error("docker must not publish sshd on the loopback")
	}
	if image, _ := services["postgres"]["image"].(string); strings.HasPrefix(image, "docker.io/") || image == "" {
		t.Errorf("postgres image = %q, want the short Docker name untouched", image)
	}
}

func TestGenerateCompose_RootlessPodmanMapsHostUserAndPublishesSSH(t *testing.T) {
	devcontainer := composeServices(t, makeConfig(withEngine(rootlessPodman)))["devcontainer-ssh"]

	if got := devcontainer["userns_mode"]; got != types.KeepIDUserNamespace {
		t.Errorf("userns_mode = %v, want %q", got, types.KeepIDUserNamespace)
	}
	if got := devcontainer["user"]; got != types.RootUser {
		t.Errorf("user = %v, want %q: keep-id would otherwise start the entrypoint as the host user", got, types.RootUser)
	}
	if !containsString(stringList(t, devcontainer, "ports"), types.SSHLoopbackPublishing) {
		t.Errorf("ports = %v, want %q published", devcontainer["ports"], types.SSHLoopbackPublishing)
	}
}

func TestGenerateCompose_RootfulPodmanNeedsNeitherKeepIDNorAPublishedSSHPort(t *testing.T) {
	devcontainer := composeServices(t, makeConfig(withEngine(rootfulPodman)))["devcontainer-ssh"]

	if _, set := devcontainer["userns_mode"]; set {
		t.Errorf("rootful podman must not get userns_mode, got %v", devcontainer["userns_mode"])
	}
	if _, set := devcontainer["user"]; set {
		t.Errorf("rootful podman must not override the user, got %v", devcontainer["user"])
	}
	if containsString(stringList(t, devcontainer, "ports"), types.SSHLoopbackPublishing) {
		t.Error("rootful podman reaches the container by IP; sshd must not be published")
	}
}

func TestGenerateCompose_PodmanQualifiesServiceImagesButNotTheDevcontainerImage(t *testing.T) {
	services := composeServices(t, makeConfig(withEngine(rootfulPodman), withPostgresAndDod))

	if image, _ := services["postgres"]["image"].(string); !strings.HasPrefix(image, "docker.io/library/postgres:") {
		t.Errorf("postgres image = %q, want it qualified with docker.io/library/", image)
	}
	if image, _ := services["devcontainer-ssh"]["image"].(string); image != "devcontainer-ssh:local" {
		t.Errorf("devcontainer image = %q, want it untouched", image)
	}
}

func TestGenerateCompose_DodMountsTheEnginesOwnSocket(t *testing.T) {
	cases := []struct {
		name   string
		traits types.EngineTraits
		want   string
	}{
		{"docker", types.EngineTraits{}, "/var/run/docker.sock:/var/run/docker.sock"},
		{"rootful podman", rootfulPodman, "/run/podman/podman.sock:/var/run/docker.sock"},
		{"rootless podman", rootlessPodman, "/run/user/1000/podman/podman.sock:/var/run/docker.sock"},
		{"podman with no socket reported", types.EngineTraits{IsPodman: true}, "/var/run/docker.sock:/var/run/docker.sock"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			devcontainer := composeServices(t, makeConfig(withEngine(c.traits), withPostgresAndDod))["devcontainer-ssh"]
			if !containsString(stringList(t, devcontainer, "volumes"), c.want) {
				t.Errorf("volumes = %v, want %q", devcontainer["volumes"], c.want)
			}
		})
	}
}

func TestGenerateDockerfile_PodmanQualifiesTheBaseImage(t *testing.T) {
	podman := mustGenerateDockerfile(t, makeConfig(withEngine(rootfulPodman)))
	assertContainsStr(t, podman, "FROM docker.io/library/ubuntu:24.04", "podman dockerfile")
	assertNotContainsStr(t, podman, "FROM ubuntu:", "podman dockerfile")

	docker := mustGenerateDockerfile(t, makeConfig())
	assertContainsStr(t, docker, "FROM ubuntu:24.04", "docker dockerfile")
}

func TestQualifyImageReference(t *testing.T) {
	cases := []struct{ in, want string }{
		{"postgres:16", "docker.io/library/postgres:16"},
		{"caddy:alpine", "docker.io/library/caddy:alpine"},
		{"bitnami/redis:7", "docker.io/bitnami/redis:7"},
		{"ghcr.io/joacohbc/devcontainer-utils:latest", "ghcr.io/joacohbc/devcontainer-utils:latest"},
		{"localhost/devcontainer-cli/abc:latest", "localhost/devcontainer-cli/abc:latest"},
		{"registry:5000/team/app:1", "registry:5000/team/app:1"},
		{"docker.io/library/ubuntu:24.04", "docker.io/library/ubuntu:24.04"},
		{"", ""},
	}
	for _, c := range cases {
		if got := domain.QualifyImageReference(c.in); got != c.want {
			t.Errorf("QualifyImageReference(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripLocalRegistry(t *testing.T) {
	if got := domain.StripLocalRegistry("localhost/devcontainer-cli/abc:latest"); got != "devcontainer-cli/abc:latest" {
		t.Errorf("StripLocalRegistry = %q", got)
	}
	if got := domain.StripLocalRegistry("devcontainer-cli/abc:latest"); got != "devcontainer-cli/abc:latest" {
		t.Errorf("StripLocalRegistry changed a reference with no localhost prefix: %q", got)
	}
}

func TestIsLocalImage_RecognisesPodmansLocalhostPrefix(t *testing.T) {
	if !domain.IsLocalImage("localhost/" + domain.FingerprintTag("abcdef123456789012")) {
		t.Error("a localhost/-prefixed build must still count as ours")
	}
	if domain.IsLocalImage("localhost/someone-else/app:latest") {
		t.Error("a foreign localhost image must not count as ours")
	}
}
