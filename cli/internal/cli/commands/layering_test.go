package commands

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Commands are a thin shell over internal/service: they never shell out or
// reach the infra adapters themselves (AGENTS.md, 4-layer architecture).
func TestCommandsDoNotImportProcessOrInfraAdapters(t *testing.T) {
	forbidden := []string{
		"os/exec",
		"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker",
		"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/git",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(fset, file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if path == bad {
					t.Errorf("%s imports %s; move that work into internal/service", file, path)
				}
			}
		}
	}
}
