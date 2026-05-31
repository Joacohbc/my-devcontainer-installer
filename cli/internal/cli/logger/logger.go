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
