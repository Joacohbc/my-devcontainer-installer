package dockerfile

import (
	"fmt"
	"strings"
)

var rcFiles = []string{".zshrc", ".bashrc", ".profile"}

const devuserHome = "/home/devuser"

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
	return fmt.Sprintf(
		`RUN su - devuser -c "printf '%%s\n' %s > %s"
RUN su - devuser -c "for f in %s; do touch %s/\$f && echo '%s' >> %s/\$f; done"`,
		argsStr, initFile,
		rcList, devuserHome, sourceLine, devuserHome,
	)
}
