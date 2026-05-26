package domain_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func makeConfig(overrides ...func(*core.DevcontainerConfig)) *core.DevcontainerConfig {
	cfg := &core.DevcontainerConfig{
		Mode:      core.BuildModeLocalCached,
		Image:     "devcontainer-ssh:local",
		Workspace: "devcontainer",
		Dockerfile: core.DockerfileConfig{
			Modules: []core.SelectedModule{},
		},
		Compose: core.ComposeConfig{
			Services: []any{},
			Subnet:   "172.25.0.0/24",
		},
		Env: map[string]string{},
	}
	for _, fn := range overrides {
		fn(cfg)
	}
	return cfg
}

func mustGenerateDockerfile(t *testing.T, cfg *core.DevcontainerConfig) string {
	t.Helper()
	out, err := domain.GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}
	return out
}

func mustGenerateCompose(t *testing.T, cfg *core.DevcontainerConfig) string {
	t.Helper()
	out, err := domain.GenerateCompose(cfg)
	if err != nil {
		t.Fatalf("GenerateCompose failed: %v", err)
	}
	return out
}

func assertContainsStr(t *testing.T, haystack, needle, context string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("[%s] expected output to contain %q", context, needle)
	}
}

func assertNotContainsStr(t *testing.T, haystack, needle, context string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("[%s] expected output NOT to contain %q", context, needle)
	}
}

func TestGenerateDockerfile_MinimalHasBaseAndCleanup(t *testing.T) {
	df := mustGenerateDockerfile(t, makeConfig())
	assertContainsStr(t, df, "FROM ubuntu:24.04", "minimal dockerfile")
	assertContainsStr(t, df, `ENTRYPOINT ["/entrypoint.sh"]`, "minimal dockerfile")
	assertContainsStr(t, df, "AUTO-GENERATED", "minimal dockerfile")
}

func TestGenerateDockerfile_AllSelectedModules(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{
			{ID: "dod"},
			{ID: "java-temurin"},
			{ID: "python"},
			{ID: "sqlite"},
			{ID: "go"},
			{ID: "dbclients"},
			{ID: "nodejs"},
			{ID: "bun"},
			{ID: "tmux"},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "docker-ce-cli", "full dockerfile")
	assertContainsStr(t, df, "temurin-17-jdk", "full dockerfile")
	assertContainsStr(t, df, "python3-pip", "full dockerfile")
	assertContainsStr(t, df, "sqlite3", "full dockerfile")
	assertContainsStr(t, df, "install_golang", "full dockerfile")
	assertContainsStr(t, df, "mongodb-mongosh", "full dockerfile")
	assertContainsStr(t, df, "nvm install --lts", "full dockerfile")
	assertContainsStr(t, df, "bun.sh/install", "full dockerfile")
	assertContainsStr(t, df, "tmux", "full dockerfile")
}

func TestGenerateDockerfile_TmuxModule(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "tmux"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "RUN apt-get update && apt-get install -y tmux", "tmux")
}

func TestGenerateDockerfile_PythonWithUvByDefault(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "python"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "astral.sh/uv/install.sh", "python default")
	assertContainsStr(t, df, ".python_init.sh", "python default")
	assertContainsStr(t, df, "for f in .zshrc .bashrc .profile", "python default")
}

func TestGenerateDockerfile_PythonWithoutUv(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{
			{ID: "python", Options: map[string]any{"uv": false}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertNotContainsStr(t, df, "astral.sh", "python no-uv")
}

func TestGenerateDockerfile_BunShellInit(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "bun"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "bun.sh/install", "bun")
	assertContainsStr(t, df, ".bun_init.sh", "bun")
	assertContainsStr(t, df, "for f in .zshrc .bashrc .profile", "bun")
}

func TestGenerateDockerfile_PnpmAutoAddNodejs(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "pnpm"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "nvm install", "pnpm pulls nodejs")
	assertContainsStr(t, df, "get.pnpm.io/install.sh", "pnpm")
}

func TestGenerateDockerfile_NodejsFnmManager(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{
			{ID: "nodejs", Options: map[string]any{"manager": "fnm"}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "fnm.vercel.app/install", "fnm")
	assertContainsStr(t, df, "fnm install --lts", "fnm")
	assertNotContainsStr(t, df, "nvm-sh/nvm", "fnm")
}

func TestGenerateDockerfile_NodejsFnmSpecificVersion(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{
			{ID: "nodejs", Options: map[string]any{"manager": "fnm", "version": "22"}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "fnm install 22", "fnm version 22")
}

func TestGenerateDockerfile_RemoteModeReturnsEmpty(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Mode = core.BuildModeRemote
		c.Remote = &core.RemoteConfig{Variant: "python"}
	})
	out, err := domain.GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string for remote mode, got %q", out)
	}
}

func TestGenerateCompose_ValidYAMLWithExpectedServices(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{"mongo", "tunnel"}
		c.Compose.Subnet = "10.0.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, yml)
	}
	services := parsed["services"].(map[string]any)
	if services["devcontainer-ssh"] == nil {
		t.Error("expected devcontainer-ssh service")
	}
	if services["mongo"] == nil {
		t.Error("expected mongo service")
	}
	if services["tunnel"] == nil {
		t.Error("expected tunnel service")
	}
	if services["redis"] != nil {
		t.Error("expected redis to be absent")
	}
	if services["postgres"] != nil {
		t.Error("expected postgres to be absent")
	}
}

func TestGenerateCompose_DependsOnEnabledDBOnly(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{"mongo"}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	dependsOn := devSvc["depends_on"]
	if dependsOn == nil {
		t.Fatal("expected depends_on to be set when mongo is enabled")
	}
	deps, ok := dependsOn.([]any)
	if !ok || len(deps) != 1 || deps[0] != "mongo" {
		t.Errorf("expected depends_on=[mongo], got %v", dependsOn)
	}
}

func TestGenerateCompose_NoDependsOnWithoutDB(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	if devSvc["depends_on"] != nil {
		t.Errorf("expected no depends_on, got %v", devSvc["depends_on"])
	}
}

func TestGenerateCompose_StaticIPFromSubnet(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/28"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	networks := devSvc["networks"].(map[string]any)
	networkName := "devcontainer-network"
	netEntry := networks[networkName].(map[string]any)
	ipv4 := netEntry["ipv4_address"].(string)
	if ipv4 != "${DEVCONTAINER_IP:-172.25.0.14}" {
		t.Errorf("expected ipv4_address=${DEVCONTAINER_IP:-172.25.0.14}, got %q", ipv4)
	}
}

func TestGenerateCompose_NoDockerSocketByDefault(t *testing.T) {
	cfg := makeConfig()
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	volumes, _ := devSvc["volumes"].([]any)
	for _, v := range volumes {
		if strings.Contains(v.(string), "docker.sock") {
			t.Error("docker.sock must not be mounted by default")
		}
	}
	if devSvc["environment"] != nil {
		t.Errorf("expected no environment in default mode, got %v", devSvc["environment"])
	}
}

func TestGenerateCompose_DindModeAutoProvisions(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{
			map[string]any{"id": "devcontainer", "options": map[string]any{"dockerSocket": "dind"}},
			map[string]any{"id": "postgres", "options": map[string]any{}},
		}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, yml)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)

	volumes, _ := devSvc["volumes"].([]any)
	for _, v := range volumes {
		if strings.Contains(v.(string), "docker.sock") {
			t.Error("docker.sock must not be mounted in dind mode")
		}
	}

	envList, ok := devSvc["environment"].([]any)
	if !ok {
		t.Fatal("expected environment list in dind mode")
	}
	foundDockerHost := false
	for _, e := range envList {
		if e.(string) == "DOCKER_HOST=tcp://docker-dind:2375" {
			foundDockerHost = true
		}
	}
	if !foundDockerHost {
		t.Error("expected DOCKER_HOST=tcp://docker-dind:2375 in dind mode")
	}

	depsList, ok := devSvc["depends_on"].([]any)
	if !ok {
		t.Fatal("expected depends_on list in dind mode")
	}
	foundDind := false
	for _, d := range depsList {
		if d.(string) == "docker-dind" {
			foundDind = true
		}
	}
	if !foundDind {
		t.Error("expected docker-dind in depends_on")
	}

	devNetworks := devSvc["networks"].(map[string]any)
	if _, ok := devNetworks["devcontainer-network"]; !ok {
		t.Error("expected devcontainer-network in devcontainer networks")
	}
	if _, ok := devNetworks["devcontainer-engine-network"]; !ok {
		t.Error("expected devcontainer-engine-network in devcontainer networks in dind mode")
	}

	dindSvc, ok := services["docker-dind"]
	if !ok {
		t.Fatal("expected docker-dind service to be auto-provisioned")
	}
	dind := dindSvc.(map[string]any)
	image := dind["image"].(string)
	if !strings.Contains(image, "dind-rootless") {
		t.Errorf("expected dind-rootless image, got %q", image)
	}
	if dind["privileged"] != true {
		t.Error("expected dind service to be privileged")
	}

	dindNetworks, _ := dind["networks"].([]any)
	foundEngineNet := false
	for _, n := range dindNetworks {
		if n.(string) == "devcontainer-engine-network" {
			foundEngineNet = true
		}
	}
	if !foundEngineNet {
		t.Error("expected docker-dind to be on engine-network")
	}

	networks := parsed["networks"].(map[string]any)
	if networks["devcontainer-engine-network"] == nil {
		t.Error("expected devcontainer-engine-network to be declared")
	}

	pgSvc := services["postgres"].(map[string]any)
	pgNetworks := pgSvc["networks"]
	pgNetStr := ""
	switch v := pgNetworks.(type) {
	case []any:
		for _, n := range v {
			pgNetStr += n.(string) + ","
		}
	case map[string]any:
		for k := range v {
			pgNetStr += k + ","
		}
	}
	if strings.Contains(pgNetStr, "engine-network") {
		t.Error("postgres should not be on the engine-network")
	}
}

func TestGenerateCompose_NoDindWithSocketNone(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{
			map[string]any{"id": "devcontainer", "options": map[string]any{"dockerSocket": "none"}},
		}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	if services["docker-dind"] != nil {
		t.Error("expected docker-dind NOT to be present in none mode")
	}
	devSvc := services["devcontainer-ssh"].(map[string]any)
	volumes, _ := devSvc["volumes"].([]any)
	for _, v := range volumes {
		if strings.Contains(v.(string), "docker.sock") {
			t.Error("docker.sock must not be mounted in none mode")
		}
	}
	if devSvc["environment"] != nil {
		t.Error("expected no environment in none mode")
	}
}

func TestGenerateCompose_SocketModeMountsDockerSock(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{
			map[string]any{"id": "devcontainer", "options": map[string]any{"dockerSocket": "socket"}},
		}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	volumes, _ := devSvc["volumes"].([]any)
	foundSocket := false
	for _, v := range volumes {
		if v.(string) == "/var/run/docker.sock:/var/run/docker.sock" {
			foundSocket = true
		}
	}
	if !foundSocket {
		t.Error("expected /var/run/docker.sock to be mounted in socket mode")
	}
	if devSvc["environment"] != nil {
		t.Error("expected no DOCKER_HOST in socket mode")
	}
	devNetworks := devSvc["networks"].(map[string]any)
	if _, hasEngineNet := devNetworks["devcontainer-engine-network"]; hasEngineNet {
		t.Error("expected no engine-network in socket mode")
	}
}

func TestGenerateCompose_VolumesHaveWorkspacePrefix(t *testing.T) {
	cfg := makeConfig()
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	volumes := parsed["volumes"].(map[string]any)
	for k := range volumes {
		if !strings.HasPrefix(k, "devcontainer_") {
			t.Errorf("volume %q should have workspace prefix 'devcontainer_'", k)
		}
	}
}

func TestGenerateCompose_NetworkNamedAfterWorkspace(t *testing.T) {
	cfg := makeConfig()
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	networks := parsed["networks"].(map[string]any)
	if networks["devcontainer-network"] == nil {
		t.Error("expected devcontainer-network in networks")
	}
}

func TestGenerateCompose_RemoteModeOmitsBuild(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Mode = core.BuildModeRemote
		c.Image = "ghcr.io/joacohbc/devcontainer-node-java-temurin:latest"
		c.Remote = &core.RemoteConfig{Variant: "node-java-temurin"}
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	if devSvc["build"] != nil {
		t.Errorf("expected no build in remote mode, got %v", devSvc["build"])
	}
	if devSvc["image"] != "ghcr.io/joacohbc/devcontainer-node-java-temurin:latest" {
		t.Errorf("expected remote image, got %v", devSvc["image"])
	}
}

func TestGenerateCompose_RemoteModeWithDBService(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Mode = core.BuildModeRemote
		c.Image = "ghcr.io/joacohbc/devcontainer-ssh:latest"
		c.Remote = &core.RemoteConfig{Variant: "ssh"}
		c.Compose.Services = []any{"mongo"}
		c.Compose.Subnet = "172.25.0.0/24"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	if services["mongo"] == nil {
		t.Error("expected mongo service in remote mode with DB selected")
	}
}

func TestResolveRemoteImage_SSHVariant(t *testing.T) {
	got := domain.ResolveRemoteImage("ssh", "ghcr.io/joacohbc/")
	want := "ghcr.io/joacohbc/devcontainer-ssh:latest"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveRemoteImage_NonSSHVariant(t *testing.T) {
	got := domain.ResolveRemoteImage("bun", "ghcr.io/joacohbc/")
	want := "ghcr.io/joacohbc/devcontainer-bun:latest"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveRemoteImage_DefaultRegistry(t *testing.T) {
	got := domain.ResolveRemoteImage("python", "")
	if !strings.HasSuffix(got, "devcontainer-python:latest") {
		t.Errorf("expected suffix devcontainer-python:latest, got %q", got)
	}
}

func TestGenerateCompose_FingerprintUsedAsImageName(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Mode = core.BuildModeLocalCached
		c.Fingerprint = "abc123def4567890abc123def4567890"
		c.Image = "should-not-be-used:tag"
	})
	yml := mustGenerateCompose(t, cfg)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yml), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	services := parsed["services"].(map[string]any)
	devSvc := services["devcontainer-ssh"].(map[string]any)
	if devSvc["image"] != "devcontainer-cli/abc123def456:latest" {
		t.Errorf("expected fingerprint image, got %v", devSvc["image"])
	}
	if devSvc["build"] != "." {
		t.Errorf("expected build='.' in local-cached mode, got %v", devSvc["build"])
	}
}

func TestGenerateEnv_IncludesEnvVars(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Env = map[string]string{"TUNNEL_TOKEN": "abc"}
		c.Compose.Services = []any{}
		c.Compose.Subnet = "10.0.0.0/8"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "TUNNEL_TOKEN=abc", "env")
	assertContainsStr(t, env, "DOCKER_SUBNET=10.0.0.0/8", "env")
}

func TestGenerateEnv_DerivesIPFromDockerSubnetOverride(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Env = map[string]string{"DOCKER_SUBNET": "172.26.0.0/24"}
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/28"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "DOCKER_SUBNET=172.26.0.0/24", "env override")
	assertContainsStr(t, env, "DEVCONTAINER_IP=172.26.0.254", "env override ip")
}

func TestGenerateEnv_WritesBothSubnetAndIP(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/28"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "DOCKER_SUBNET=172.25.0.0/28", "env subnet")
	assertContainsStr(t, env, "DEVCONTAINER_IP=172.25.0.14", "env ip")
}

func TestGenerateDockerfile_AiClisDefaultsAllTools(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "ai-clis"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-claude-code.sh", "ai-clis defaults")
	assertContainsStr(t, df, "install-opencode.sh", "ai-clis defaults")
	assertContainsStr(t, df, "install-codex-cli.sh", "ai-clis defaults")
	assertContainsStr(t, df, "install-antigravity.sh", "ai-clis defaults")
	assertContainsStr(t, df, "install-copilot.sh", "ai-clis defaults")
	assertContainsStr(t, df, "/home/devuser/post-script/", "ai-clis post-script dir")
	assertContainsStr(t, df, "chmod +x /home/devuser/post-script/*.sh", "ai-clis chmod")
}

func TestGenerateDockerfile_AiClisSubset(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{
			{ID: "ai-clis", Options: map[string]any{"tools": []any{"claude-code"}}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-claude-code.sh", "ai-clis subset")
	assertNotContainsStr(t, df, "install-opencode.sh", "ai-clis subset")
	assertNotContainsStr(t, df, "install-codex-cli.sh", "ai-clis subset")
	assertNotContainsStr(t, df, "install-antigravity.sh", "ai-clis subset")
	assertNotContainsStr(t, df, "install-copilot.sh", "ai-clis subset")
}

func TestGenerateDockerfile_AiClisPullsPnpmAndGithubCli(t *testing.T) {
	cfg := makeConfig(func(c *core.DevcontainerConfig) {
		c.Dockerfile.Modules = []core.SelectedModule{{ID: "ai-clis"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "get.pnpm.io/install.sh", "ai-clis requires pnpm")
	assertContainsStr(t, df, "cli.github.com/packages", "ai-clis requires github-cli")
}
