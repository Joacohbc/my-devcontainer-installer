package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var rcFiles = []string{".zshrc", ".bashrc", ".profile"}

const devuserHome = types.DevUserHome

func emitShellInit(initFileName string, lines []string) string {
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
	rcList := strings.Join(rcFiles, " ")
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
	lines := make([]string, len(sources))
	for i, s := range sources {
		if s.Guarded {
			// `if …; then …; fi` rather than `[ … ] && …` so a missing file leaves
			// the rc file's exit status at 0 instead of 1.
			lines[i] = `if [ -r \$HOME/` + s.File + ` ]; then . \$HOME/` + s.File + `; fi`
		} else {
			lines[i] = `. \$HOME/` + s.File
		}
	}
	echos := make([]string, len(lines))
	for i, l := range lines {
		echos[i] = fmt.Sprintf(`echo '%s' >> %s/\$f`, l, devuserHome)
	}
	rcList := strings.Join(rcFiles, " ")
	return fmt.Sprintf(
		`RUN su - devuser -c "for f in %s; do touch %s/\$f && %s; done"`,
		rcList, devuserHome, strings.Join(echos, " && "),
	)
}
