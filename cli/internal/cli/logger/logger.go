package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

var std = log.NewWithOptions(os.Stderr, log.Options{
	ReportTimestamp: false,
	ReportCaller:    false,
	Level:           log.WarnLevel,
	Prefix:          "devcontainer",
})

func Std() *log.Logger { return std }

func SetLevel(lvl log.Level) { std.SetLevel(lvl) }

// ParseLevel converts the string typed by the user into the log.Level the
// library understands.
//
// log.Level is a `type Level int` (not a string): DebugLevel=-4, InfoLevel=0,
// WarnLevel=4, ErrorLevel=8, FatalLevel=12. They are numbers because a log
// level is ordered: the logger prints a message only if its level is >= the
// configured level (e.g. with Info, Debug=-4 is dropped and Warn=4 is shown).
// That >= comparison is trivial with integers and not directly possible with
// strings. The gaps between values leave room for intermediate levels without
// renumbering. The string is just the human-facing side (String()/ParseLevel)
// for I/O with humans.
func ParseLevel(s string) (log.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return log.DebugLevel, nil
	case "info":
		return log.InfoLevel, nil
	case "warn", "warning":
		return log.WarnLevel, nil
	case "error":
		return log.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s (debug|info|warn|error)", s)
	}
}
