package logger_test

import (
	"testing"

	"github.com/charmbracelet/log"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want log.Level
		err  bool
	}{
		{"debug", log.DebugLevel, false},
		{"info", log.InfoLevel, false},
		{"warn", log.WarnLevel, false},
		{"warning", log.WarnLevel, false},
		{"error", log.ErrorLevel, false},
		{"DEBUG", log.DebugLevel, false},
		{"trace", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := logger.ParseLevel(c.in)
		if (err != nil) != c.err {
			t.Errorf("%q: err=%v want=%v", c.in, err, c.err)
		}
		if !c.err && got != c.want {
			t.Errorf("%q: got=%v want=%v", c.in, got, c.want)
		}
	}
}

func TestSetLevel(t *testing.T) {
	orig := logger.Std().GetLevel()
	defer logger.SetLevel(orig)

	logger.SetLevel(log.DebugLevel)
	if logger.Std().GetLevel() != log.DebugLevel {
		t.Fatal("level not changed")
	}
}
