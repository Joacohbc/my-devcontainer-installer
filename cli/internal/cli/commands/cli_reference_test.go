package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func renderCLIReference(root *cobra.Command) string {
	var sb strings.Builder

	sb.WriteString("# Referencia Automática de Comandos y Banderas\n\n")
	sb.WriteString("> Este documento es generado automáticamente a partir del árbol de comandos de Cobra de `devcontainer-cli`.\n")
	sb.WriteString("> Para actualizarlo tras modificar comandos o banderas, ejecute: `UPDATE_CLI_REF=1 go test ./internal/cli/commands -run TestCLIReference_UpToDate`.\n\n")

	// Collect commands in depth-first or alphabetical order.
	var collectCmds func(cmd *cobra.Command) []*cobra.Command
	collectCmds = func(cmd *cobra.Command) []*cobra.Command {
		var list []*cobra.Command
		list = append(list, cmd)

		children := cmd.Commands()
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name() < children[j].Name()
		})

		for _, child := range children {
			if child.Hidden {
				continue
			}
			list = append(list, collectCmds(child)...)
		}
		return list
	}

	allCmds := collectCmds(root)

	for _, cmd := range allCmds {
		cmdPath := cmd.CommandPath()
		sb.WriteString(fmt.Sprintf("## `%s`\n\n", cmdPath))

		short := strings.TrimSpace(cmd.Short)
		if short != "" {
			sb.WriteString(short + "\n\n")
		}

		// Flags: local and persistent
		var flags []*pflag.Flag
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden {
				flags = append(flags, f)
			}
		})

		sort.Slice(flags, func(i, j int) bool {
			return flags[i].Name < flags[j].Name
		})

		if len(flags) > 0 {
			sb.WriteString("### Banderas\n\n")
			sb.WriteString("| Bandera | Atajo | Tipo | Por defecto | Descripción |\n")
			sb.WriteString("|---|---|---|---|---|\n")

			for _, f := range flags {
				name := "--" + f.Name
				shortcut := ""
				if f.Shorthand != "" {
					shortcut = "-" + f.Shorthand
				}
				fType := f.Value.Type()
				defVal := f.DefValue
				if defVal == "" {
					defVal = "-"
				} else {
					defVal = fmt.Sprintf("`%s`", defVal)
				}
				usage := strings.ReplaceAll(f.Usage, "\n", " ")
				usage = strings.ReplaceAll(usage, "|", "\\|")

				sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", name, shortcut, fType, defVal, usage))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func TestCLIReference_UpToDate(t *testing.T) {
	root := NewRootCommand("dev")
	rendered := renderCLIReference(root)

	docPath := filepath.Join("..", "..", "..", "..", ".docs", "cli-reference.md")

	if os.Getenv("UPDATE_CLI_REF") == "1" {
		if err := os.WriteFile(docPath, []byte(rendered), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", docPath, err)
		}
		t.Logf("Successfully updated %s", docPath)
		return
	}

	existing, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v. Please run: UPDATE_CLI_REF=1 go test ./internal/cli/commands -run TestCLIReference_UpToDate", docPath, err)
	}

	if string(existing) != rendered {
		t.Fatalf(".docs/cli-reference.md is out of sync with Cobra command tree. Run UPDATE_CLI_REF=1 go test ./internal/cli/commands -run TestCLIReference_UpToDate to regenerate.")
	}
}
