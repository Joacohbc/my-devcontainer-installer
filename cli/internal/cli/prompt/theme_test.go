package prompt

import (
	"testing"
)

func TestThemeNonNil(t *testing.T) {
	th := devcontainerTheme()
	if th == nil {
		t.Fatal("theme is nil")
	}
	fg := th.Focused.Title.GetForeground()
	if fg == nil {
		t.Error("expected title foreground color to be set")
	}
}
