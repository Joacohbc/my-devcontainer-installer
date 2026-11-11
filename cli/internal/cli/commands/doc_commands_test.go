package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// repoRoot is the repository root seen from this package's directory.
var repoRoot = filepath.Join("..", "..", "..", "..")

// invocationRe finds a devcontainer-cli invocation inside a code span: the
// binary name (not part of a path or an image name) followed by its arguments.
var invocationRe = regexp.MustCompile(`(?:^|[^\w/.-])devcontainer-cli((?:[ \t]+[^\s|;&)` + "`" + `']+)*)`)

// commandWordRe is a subcommand token; anything else ends the command path.
var commandWordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// TestDocumentedInvocationsExist checks every `devcontainer-cli ...` written in
// the skill, the README, .docs/ and the commands' own Example/Long text against
// the real command tree: the subcommand path must exist and every --long flag
// must be one the command accepts. Agents copy these lines verbatim, and a
// renamed command or flag otherwise drifts silently (`image update --build`,
// `config alias sync` and `devcontainer-cli context` all survived that way).
func TestDocumentedInvocationsExist(t *testing.T) {
	root := NewRootCommand("test")
	for _, src := range documentedInvocations(t, root) {
		if problem := checkInvocation(root, src.args); problem != "" {
			t.Errorf("%s: `devcontainer-cli%s`: %s", src.where, src.args, problem)
		}
	}
}

type invocation struct{ where, args string }

func documentedInvocations(t *testing.T, root *cobra.Command) []invocation {
	t.Helper()
	var out []invocation
	var files []string
	for _, dir := range []string{"skills", ".docs"} {
		_ = filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
				files = append(files, path)
			}
			return nil
		})
	}
	files = append(files, filepath.Join(repoRoot, "README.md"), filepath.Join(repoRoot, "AGENTS.md"))
	if len(files) < 5 {
		t.Fatalf("found only %d documentation files under %s", len(files), repoRoot)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for _, line := range codeLines(string(data)) {
			for _, args := range findInvocations(line.text) {
				out = append(out, invocation{fmt.Sprintf("%s:%d", rel, line.number), args})
			}
		}
	}
	for _, cmd := range allCommands(root) {
		for _, text := range []string{cmd.Example, cmd.Long} {
			for _, line := range strings.Split(text, "\n") {
				for _, args := range findInvocations(line) {
					out = append(out, invocation{"help of '" + cmd.CommandPath() + "'", args})
				}
			}
		}
	}
	return out
}

type numberedLine struct {
	number int
	text   string
}

// codeLines returns the parts of a markdown file that hold commands: fenced
// and indented code blocks whole, and the inline `code` spans of prose lines.
// Prose itself is skipped, so "devcontainer-cli is a Go CLI" is not a command.
func codeLines(markdown string) []numberedLine {
	var out []numberedLine
	fenced := false
	for i, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
			continue
		}
		if fenced || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			out = append(out, numberedLine{i + 1, line})
			continue
		}
		parts := strings.Split(line, "`")
		for j := 1; j < len(parts); j += 2 {
			out = append(out, numberedLine{i + 1, parts[j]})
		}
	}
	return out
}

func findInvocations(text string) []string {
	for _, end := range []string{" #", " — "} {
		if i := strings.Index(text, end); i >= 0 {
			text = text[:i]
		}
	}
	var out []string
	for _, m := range invocationRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// checkInvocation walks the subcommand words of args down the tree and checks
// the --long flags that follow, up to a `--` separator.
func checkInvocation(root *cobra.Command, args string) string {
	cmd := root
	tokens := strings.Fields(args)
	i := 0
	for ; i < len(tokens) && commandWordRe.MatchString(tokens[i]); i++ {
		next := subcommand(cmd, tokens[i])
		if next == nil {
			if cmd.HasAvailableSubCommands() {
				return fmt.Sprintf("'%s' has no subcommand %q", cmd.CommandPath(), tokens[i])
			}
			break
		}
		cmd = next
	}
	for _, tok := range tokens[i:] {
		if tok == "--" {
			break
		}
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		name := strings.TrimPrefix(tok, "--")
		name, _, _ = strings.Cut(name, "=")
		if !commandWordRe.MatchString(name) || name == "help" {
			continue
		}
		if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
			return fmt.Sprintf("'%s' has no flag --%s", cmd.CommandPath(), name)
		}
	}
	return ""
}

func subcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

func allCommands(cmd *cobra.Command) []*cobra.Command {
	out := []*cobra.Command{cmd}
	for _, c := range cmd.Commands() {
		out = append(out, allCommands(c)...)
	}
	return out
}
