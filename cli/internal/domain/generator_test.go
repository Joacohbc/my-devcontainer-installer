package domain_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func makeConfig(overrides ...func(*types.DevcontainerConfig)) *types.DevcontainerConfig {
	cfg := &types.DevcontainerConfig{
		Mode:      types.BuildModeLocalCached,
		Image:     "devcontainer-ssh:local",
		Workspace: "devcontainer",
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{},
		},
		Compose: types.ComposeConfig{
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

func mustGenerateDockerfile(t *testing.T, cfg *types.DevcontainerConfig) string {
	t.Helper()
	out, err := domain.GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}
	return out
}

func mustGenerateCompose(t *testing.T, cfg *types.DevcontainerConfig) string {
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{
			{ID: "java-temurin"},
			{ID: "python"},
			{ID: "sqlite"},
			{ID: "go"},
			{ID: "postgres-client"},
			{ID: "redis-client"},
			{ID: "mysql-client"},
			{ID: "mongo-client"},
			{ID: "nodejs"},
			{ID: "bun"},
			{ID: "tmux"},
			{ID: "php"},
			{ID: "rust"},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "temurin-17-jdk", "full dockerfile")
	assertContainsStr(t, df, "python3-pip", "full dockerfile")
	assertContainsStr(t, df, "sqlite3", "full dockerfile")
	assertContainsStr(t, df, "install_golang", "full dockerfile")
	assertContainsStr(t, df, "postgresql-client", "full dockerfile")
	assertContainsStr(t, df, "redis-tools", "full dockerfile")
	assertContainsStr(t, df, "default-mysql-client", "full dockerfile")
	assertContainsStr(t, df, "mongodb-mongosh", "full dockerfile")
	assertContainsStr(t, df, "nvm install --lts", "full dockerfile")
	assertContainsStr(t, df, "bun.sh/install", "full dockerfile")
	assertContainsStr(t, df, "tmux", "full dockerfile")
	assertContainsStr(t, df, "ppa:ondrej/php", "full dockerfile")
	assertContainsStr(t, df, "rustup.rs", "full dockerfile")
}

func TestGenerateDockerfile_TmuxModule(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "tmux"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "RUN apt-get update && apt-get install -y tmux", "tmux")
}

func TestGenerateDockerfile_PythonWithUvByDefault(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "python"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "astral.sh/uv/install.sh", "python default")
	assertContainsStr(t, df, ".python_init.sh", "python default")
	assertContainsStr(t, df, "for f in .zshrc .bashrc .profile", "python default")
}

func TestGenerateDockerfile_PythonWithoutUv(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{
			{ID: "python", Options: map[string]any{"uv": false}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertNotContainsStr(t, df, "astral.sh", "python no-uv")
}

func TestGenerateDockerfile_BunShellInit(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "bun"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "bun.sh/install", "bun")
	assertContainsStr(t, df, ".bun_init.sh", "bun")
	assertContainsStr(t, df, "for f in .zshrc .bashrc .profile", "bun")
}

func TestGenerateDockerfile_PnpmAutoAddNodejs(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "pnpm"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "nvm install", "pnpm pulls nodejs")
	assertContainsStr(t, df, "get.pnpm.io/install.sh", "pnpm")
}

func TestGenerateDockerfile_NodejsFnmManager(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{
			{ID: "nodejs", Options: map[string]any{"manager": "fnm"}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "fnm.vercel.app/install", "fnm")
	assertContainsStr(t, df, "fnm install --lts", "fnm")
	assertNotContainsStr(t, df, "nvm-sh/nvm", "fnm")
}

func TestGenerateDockerfile_NodejsFnmSpecificVersion(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{
			{ID: "nodejs", Options: map[string]any{"manager": "fnm", "version": "22"}},
		}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "fnm install 22", "fnm version 22")
}

func TestGenerateDockerfile_RemoteModeReturnsEmpty(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Mode = types.BuildModeRemote
		c.Remote = &types.RemoteConfig{Variant: "python"}
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
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
	if services["mysql"] != nil {
		t.Error("expected mysql to be absent")
	}
}

func TestGenerateCompose_DependsOnEnabledDBOnly(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
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

func TestGenerateCompose_DevcontainerHasNoDockerAccess(t *testing.T) {
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
			t.Error("docker.sock must never be mounted")
		}
	}
	if devSvc["environment"] != nil {
		t.Errorf("expected no environment on devcontainer, got %v", devSvc["environment"])
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Mode = types.BuildModeRemote
		c.Image = "ghcr.io/joacohbc/devcontainer-node-java-temurin:latest"
		c.Remote = &types.RemoteConfig{Variant: "node-java-temurin"}
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Mode = types.BuildModeRemote
		c.Image = "ghcr.io/joacohbc/devcontainer-ssh:latest"
		c.Remote = &types.RemoteConfig{Variant: "ssh"}
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Mode = types.BuildModeLocalCached
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
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Env = map[string]string{"TUNNEL_TOKEN": "abc"}
		c.Compose.Services = []any{}
		c.Compose.Subnet = "10.0.0.0/8"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "TUNNEL_TOKEN=abc", "env")
	assertContainsStr(t, env, "DOCKER_SUBNET=10.0.0.0/8", "env")
}

func TestGenerateEnv_DerivesIPFromDockerSubnetOverride(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Env = map[string]string{"DOCKER_SUBNET": "172.26.0.0/24"}
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/28"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "DOCKER_SUBNET=172.26.0.0/24", "env override")
	assertContainsStr(t, env, "DEVCONTAINER_IP=172.26.0.254", "env override ip")
}

func TestGenerateEnv_WritesBothSubnetAndIP(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Compose.Services = []any{}
		c.Compose.Subnet = "172.25.0.0/28"
	})
	env := domain.GenerateEnv(cfg)
	assertContainsStr(t, env, "DOCKER_SUBNET=172.25.0.0/28", "env subnet")
	assertContainsStr(t, env, "DEVCONTAINER_IP=172.25.0.14", "env ip")
}

func TestGenerateDockerfile_ClaudeCode(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "claude-code"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-claude-code.sh", "claude-code script")
	assertContainsStr(t, df, "/home/devuser/post-script/", "post-script dir")
}

func TestGenerateDockerfile_Opencode(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "opencode"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-opencode.sh", "opencode script")
}

func TestGenerateDockerfile_CodexCli(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "codex-cli"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-codex-cli.sh", "codex-cli script")
	// codex-cli requires nodejs
	assertContainsStr(t, df, "nvm install --lts", "codex-cli requires nodejs")
}

func TestGenerateDockerfile_AntigravityCli(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "antigravity-cli"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-antigravity.sh", "antigravity-cli script")
}

func TestGenerateDockerfile_CopilotCli(t *testing.T) {
	cfg := makeConfig(func(c *types.DevcontainerConfig) {
		c.Dockerfile.Modules = []types.SelectedModule{{ID: "copilot-cli"}}
	})
	df := mustGenerateDockerfile(t, cfg)
	assertContainsStr(t, df, "install-copilot.sh", "copilot-cli script")
}
