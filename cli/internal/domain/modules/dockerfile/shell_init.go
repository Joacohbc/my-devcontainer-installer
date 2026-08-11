package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var rcFiles = []string{".zshrc", ".bashrc", ".profile"}

const devuserHome = types.DevUserHome

// emitShellInit writes a module's own init file and sources it from the rc
// files. It is for initialisation only a shell can carry out — `eval "$(fnm
// env --use-on-cd)"` installs a cd hook, `fnm use` picks a version for that one
// session. Those are actions, not values, which is why they cannot be declared.
//
// Anything that IS a value — a PATH entry, a variable a tool reads — belongs in
// the module's ProvidesEnv instead (see environment.go). Exporting it from here
// would hand it only to the processes that start a shell, so `docker exec
// <container> <tool>` would not find it. The nodejs module is the only caller
// left for exactly that reason.
func emitShellInit(initFileName string, lines []string) string {
	return emitShellInitTo(rcFiles, initFileName, lines)
}

// emitShellInitTo is emitShellInit with an explicit list of startup files. The
// environment script needs a different list from a module's shell init: it must
// also land in .zshenv, which zsh reads for every invocation rather than only
// for an interactive or login one.
func emitShellInitTo(files []string, initFileName string, lines []string) string {
	for _, l := range lines {
		if strings.Contains(l, "'") {
			panic(fmt.Sprintf("shell init line cannot contain single quotes: %s", l))
		}
	}
	initFile := devuserHome + "/" + initFileName
	args := make([]string, len(lines))
	for i, l := range lines {
		escaped := strings.ReplaceAll(l, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, `$`, `\$`)
		args[i] = "'" + escaped + "'"
	}
	argsStr := strings.Join(args, " ")
	sourceLine := `. \$HOME/` + initFileName
	rcList := strings.Join(files, " ")
	// Both steps run as devuser, so chain them in a single RUN layer: write the
	// init file, then source it from each rc file.
	return fmt.Sprintf(
		`RUN su - devuser -c "printf '%%s\n' %s > %s && for f in %s; do touch %s/\$f && echo '%s' >> %s/\$f; done"`,
		argsStr, initFile,
		rcList, devuserHome, sourceLine, devuserHome,
	)
}

// shellSource names a file to be sourced from every rc file. Guarded wraps the
// source in a readability test so an absent file never breaks the shell —
// required for files that only exist at runtime (e.g. ~/.alias.sh, which the
// entrypoint symlinks into the shared-config volume and which is simply missing
// when that volume is opted out of).
type shellSource struct {
	File    string
	Guarded bool
}

// emitShellSources appends a `. $HOME/<file>` line per source to every rc file
// WITHOUT writing the files themselves. Use it when the sourced file arrives
// some other way (a COPY at build time, or a symlink materialized by the
// entrypoint) instead of being generated from literal lines like emitShellInit
// does. All sources are appended in one RUN layer, in the given order — later
// sources win, since a shell keeps the last definition of an alias/function.
func emitShellSources(sources ...shellSource) string {
	echos := make([]string, len(sources))
	for i, s := range sources {
		target := `\$HOME/` + s.File
		sourceLine := ". " + target
		if s.Guarded {
			// `if …; then …; fi` rather than `[ … ] && …` so a missing file leaves
			// the rc file's exit status at 0 instead of 1.
			sourceLine = "if [ -r " + target + " ]; then " + sourceLine + "; fi"
		}
		echos[i] = fmt.Sprintf(`echo '%s' >> %s/\$f`, sourceLine, devuserHome)
	}
	rcList := strings.Join(rcFiles, " ")
	return fmt.Sprintf(
		`RUN su - devuser -c "for f in %s; do touch %s/\$f && %s; done"`,
		rcList, devuserHome, strings.Join(echos, " && "),
	)
}
