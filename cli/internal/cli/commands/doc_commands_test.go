package commands

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// docsRepoRoot is the repository root seen from this package's directory.
var docsRepoRoot = filepath.Join("..", "..", "..", "..")

// invocationRe finds a devcontainer-cli invocation: the binary name (not part
// of a path or an image name) followed by its arguments, up to a shell
// operator, a quote or a backtick.
var invocationRe = regexp.MustCompile(`(?:^|[^\w/.-])devcontainer-cli((?:[ \t]+[^\s|;&)"'` + "`" + `]+)*)`)

// commandWordRe is a subcommand token; anything else ends the command path.
var commandWordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// quotedSpanRe is a single-quoted span in a Go message, the way runtime hints
// name a command ("run 'access ssh --setup' first").
var quotedSpanRe = regexp.MustCompile(`'([^'\n]+)'`)

// TestDocumentedInvocationsExist checks every `devcontainer-cli ...` written in
// the skill, the README, AGENTS.md, .docs/ and the commands' own Example, Long
// and flag help against the real command tree: the subcommand path must exist
// and every flag must be one the command accepts. Agents copy these lines
// verbatim, and a renamed command or flag otherwise drifts silently.
//
// .agents/ is not scanned on purpose: its skills quote retired spellings as
// examples of what not to write.
func TestDocumentedInvocationsExist(t *testing.T) {
	root := NewRootCommand("test")
	var found []invocation
	for _, path := range markdownSources(t) {
		found = append(found, fileInvocations(t, path)...)
	}
	for _, cmd := range allCommands(root) {
		found = append(found, helpInvocations(cmd)...)
	}
	for _, inv := range found {
		if problem := checkInvocation(root, inv.args); problem != "" {
			t.Errorf("%s: `devcontainer-cli%s`: %s", inv.where, inv.args, problem)
		}
	}
}

// TestRuntimeHintsNameRealCommands checks the commands that error messages,
// console output and flag help name in single quotes ("run 'access ssh --setup' first"). A
// hint is the documentation an agent reads at the moment it is stuck, and a
// bare 'ssh --setup' that no longer exists strands it. A quoted span starting
// with a command name must be a full path from the root.
func TestRuntimeHintsNameRealCommands(t *testing.T) {
	root := NewRootCommand("test")
	nested := nestedCommandNames(root)
	for _, hint := range append(runtimeHints(t), flagUsages(root)...) {
		for _, m := range quotedSpanRe.FindAllStringSubmatch(hint.text, -1) {
			if problem := checkHintSpan(root, nested, m[1]); problem != "" {
				t.Errorf("%s: '%s': %s", hint.where, m[1], problem)
			}
		}
	}
}

func checkHintSpan(root *cobra.Command, nested map[string]bool, span string) string {
	first := strings.Fields(span + " ")[0]
	switch {
	case first == "devcontainer-cli":
		return checkInvocation(root, strings.TrimPrefix(span, first))
	case subcommand(root, first) != nil:
		return checkInvocation(root, " "+span)
	case nested[first] && !openSSHInvocation(span):
		return "names a command without its group; write the full path from the root"
	}
	return ""
}

type invocation struct{ where, args string }

type hintText struct{ where, text string }

func markdownSources(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"skills", ".docs"} {
		before := len(files)
		err := filepath.WalkDir(filepath.Join(docsRepoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
		if len(files) == before {
			t.Fatalf("no markdown found under %s: the scan would silently check nothing there", dir)
		}
	}
	return append(files, filepath.Join(docsRepoRoot, "README.md"), filepath.Join(docsRepoRoot, "AGENTS.md"))
}

func fileInvocations(t *testing.T, path string) []invocation {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(docsRepoRoot, path)
	var out []invocation
	for _, line := range codeLines(string(data)) {
		where := fmt.Sprintf("%s:%d", rel, line.number)
		out = append(out, invocationsIn(where, line.text)...)
	}
	return out
}

func helpInvocations(cmd *cobra.Command) []invocation {
	where := "help of '" + cmd.CommandPath() + "'"
	var out []invocation
	for _, line := range strings.Split(cmd.Example+"\n"+cmd.Long, "\n") {
		out = append(out, invocationsIn(where, line)...)
	}
	return out
}

func invocationsIn(where, text string) []invocation {
	var out []invocation
	for _, m := range invocationRe.FindAllStringSubmatch(text, -1) {
		out = append(out, invocation{where, cutAtProse(m[1])})
	}
	return out
}

// cutAtProse drops what follows a shell comment or an em dash inside one
// invocation's arguments: "devcontainer-cli up — starts it" is a command
// followed by an explanation.
func cutAtProse(args string) string {
	for _, end := range []string{" #", " —"} {
		if i := strings.Index(args, end); i >= 0 {
			args = args[:i]
		}
	}
	return args
}

type numberedLine struct {
	number int
	text   string
}

// codeLines returns the parts of a markdown file that hold commands: fenced
// (``` or ~~~) and indented code blocks whole, and the inline `code` spans of
// prose lines. Prose itself is skipped, so "devcontainer-cli is a Go CLI" is
// not a command.
//
// Known limits, accepted for a lint: a list item continued with 4+ spaces is
// read as code, and a span delimited by double backticks is split wrongly.
func codeLines(markdown string) []numberedLine {
	var out []numberedLine
	fenced := false
	for i, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
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

// checkInvocation walks the subcommand words of args down the tree, after any
// leading root flags, and checks the flags that follow, up to a `--`.
func checkInvocation(root *cobra.Command, args string) string {
	tokens := skipLeadingFlags(root, strings.Fields(strings.ReplaceAll(args, `"`, "")))
	cmd, rest, problem := walkSubcommands(root, tokens)
	if problem != "" || cmd.DisableFlagParsing {
		return problem
	}
	return checkFlags(cmd, rest)
}

// skipLeadingFlags drops root flags written before the subcommand, with the
// value of any that takes one ("--profile python workspace init").
func skipLeadingFlags(root *cobra.Command, tokens []string) []string {
	for len(tokens) > 0 && strings.HasPrefix(tokens[0], "-") {
		name, _ := longFlagName(tokens[0])
		f := lookupFlag(root, name, "")
		takesValue := f != nil && f.NoOptDefVal == "" && !strings.Contains(tokens[0], "=")
		tokens = tokens[1:]
		if takesValue && len(tokens) > 0 {
			tokens = tokens[1:]
		}
	}
	return tokens
}

func walkSubcommands(root *cobra.Command, tokens []string) (*cobra.Command, []string, string) {
	cmd := root
	for i, tok := range tokens {
		if !commandWordRe.MatchString(tok) {
			return cmd, tokens[i:], ""
		}
		next := subcommand(cmd, tok)
		if next != nil {
			cmd = next
			continue
		}
		if cmd.HasAvailableSubCommands() {
			return cmd, nil, fmt.Sprintf("'%s' has no subcommand %q", cmd.CommandPath(), tok)
		}
		return cmd, tokens[i:], ""
	}
	return cmd, nil, ""
}

func checkFlags(cmd *cobra.Command, tokens []string) string {
	for _, tok := range tokens {
		if tok == "--" {
			return ""
		}
		if name, ok := longFlagName(tok); ok && lookupFlag(cmd, name, "") == nil {
			return fmt.Sprintf("'%s' has no flag --%s", cmd.CommandPath(), name)
		}
		if short, ok := shortFlagName(tok); ok && lookupFlag(cmd, "", short) == nil {
			return fmt.Sprintf("'%s' has no flag -%s", cmd.CommandPath(), short)
		}
	}
	return ""
}

func longFlagName(tok string) (string, bool) {
	if !strings.HasPrefix(tok, "--") {
		return "", false
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(tok, "--"), "=")
	return name, commandWordRe.MatchString(name) && name != "help"
}

func shortFlagName(tok string) (string, bool) {
	if len(tok) != 2 || tok[0] != '-' || tok[1] == '-' || tok[1] == 'h' {
		return "", false
	}
	return tok[1:], true
}

func lookupFlag(cmd *cobra.Command, name, short string) *pflag.Flag {
	for _, set := range []*pflag.FlagSet{cmd.Flags(), cmd.InheritedFlags(), cmd.PersistentFlags()} {
		if name != "" && set.Lookup(name) != nil {
			return set.Lookup(name)
		}
		if short != "" && set.ShorthandLookup(short) != nil {
			return set.ShorthandLookup(short)
		}
	}
	return nil
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

// nestedCommandNames is every command name that only exists below a group
// ("ssh", "down", "context"), the names a stale hint uses on their own.
func nestedCommandNames(root *cobra.Command) map[string]bool {
	names := map[string]bool{}
	for _, cmd := range allCommands(root) {
		if cmd != root && cmd.Parent() != root {
			names[cmd.Name()] = true
		}
	}
	for _, top := range root.Commands() {
		delete(names, top.Name())
	}
	return names
}

// openSSHInvocation is the one bare 'ssh <alias>' a hint may name: the
// OpenSSH client run against a managed Host alias, not a CLI command.
func openSSHInvocation(span string) bool {
	return span == "ssh %s"
}

// flagUsages returns every flag's help text, which names commands in quotes
// the same way a hint does ("Ports 'access forward' tunnels ...").
func flagUsages(root *cobra.Command) []hintText {
	var out []hintText
	for _, cmd := range allCommands(root) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			out = append(out, hintText{"flag --" + f.Name + " of '" + cmd.CommandPath() + "'", f.Usage})
		})
	}
	return out
}

// runtimeHints returns the string literals passed to fmt.Errorf and to the
// console/reporter methods in the CLI and service layers.
func runtimeHints(t *testing.T) []hintText {
	t.Helper()
	var out []hintText
	for _, dir := range []string{".", filepath.Join("..", "..", "service")} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no Go sources in %s (err %v)", dir, err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			out = append(out, hintsInFile(t, path)...)
		}
	}
	return out
}

var hintFuncs = map[string]bool{"Errorf": true, "Info": true, "Warn": true, "Success": true, "Error": true, "Fatal": true}

func hintsInFile(t *testing.T, path string) []hintText {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []hintText
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !hintFuncs[sel.Sel.Name] {
			return true
		}
		for _, arg := range call.Args {
			if text := stringConstant(arg); text != "" {
				out = append(out, hintText{fset.Position(arg.Pos()).String(), text})
			}
		}
		return true
	})
	return out
}

// stringConstant resolves a string literal or a "+" concatenation of them.
func stringConstant(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return ""
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return ""
		}
		return s
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return ""
		}
		return stringConstant(e.X) + stringConstant(e.Y)
	}
	return ""
}
