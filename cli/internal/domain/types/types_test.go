package types

import (
	"strings"
	"testing"
)

// TestEveryUICategoryHasIcon guards that each ordered UI category exposes a
// colorful icon, so no category node renders without one in the wizard.
func TestEveryUICategoryHasIcon(t *testing.T) {
	for _, c := range UICategoryOrder {
		if UICategoryIcons[c] == "" {
			t.Errorf("UI category %q has no icon", c)
		}
		if UICategoryLabels[c] == "" {
			t.Errorf("UI category %q has no label", c)
		}
	}
}

// TestUICategoryLabelPrefixesIcon verifies the label helper prepends the icon
// and falls back to the bare label for an unknown category.
func TestUICategoryLabelPrefixesIcon(t *testing.T) {
	for _, c := range UICategoryOrder {
		got := UICategoryLabel(c)
		icon := UICategoryIcons[c]
		if !strings.HasPrefix(got, icon+" ") {
			t.Errorf("UICategoryLabel(%q) = %q, want prefix %q", c, got, icon+" ")
		}
		if !strings.Contains(got, UICategoryLabels[c]) {
			t.Errorf("UICategoryLabel(%q) = %q, want it to contain label %q", c, got, UICategoryLabels[c])
		}
	}

	unknown := UICategory("does-not-exist")
	if got := UICategoryLabel(unknown); got != "" {
		t.Errorf("UICategoryLabel(unknown) = %q, want empty (no icon, no label)", got)
	}
}
