package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// Every build — project or image — stages its scripts through
// MaterializeCustomScripts, so that is where a build script that touches the
// project is reported, before docker spends minutes reaching the failing layer.
func TestMaterializeCustomScriptsWarnsOnBuildScriptTouchingProject(t *testing.T) {
	src := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(src, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	config := &types.DevcontainerConfig{}
	config.Dockerfile.Scripts = []types.CustomScript{
		{File: "setup-mcp.sh", When: types.ScriptWhenBuild, Source: write("setup-mcp.sh", "mkdir -p /workspaces/.claude\n")},
		{File: "install.sh", When: types.ScriptWhenBuild, Source: write("install.sh", "uv tool install ruff\n")},
		{File: "start.sh", When: types.ScriptWhenStart, Source: write("start.sh", `mkdir -p "$DC_WORKSPACE_DIR/.agent"`+"\n")},
	}
	rec := &warnRecorder{}

	if _, err := (GenerateService{Report: rec}).MaterializeCustomScripts(config, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if len(rec.warns) != 1 || !strings.Contains(rec.warns[0], "setup-mcp.sh") {
		t.Errorf("warnings = %q, want exactly one naming setup-mcp.sh", rec.warns)
	}
}
