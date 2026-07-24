package domain

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
)

// Every module's Category must appear in categoryOrder, or orderModulesByCategory
// would silently drop it from the resolved set.
func TestCategoryOrderCoversEveryModule(t *testing.T) {
	inOrder := make(map[string]bool, len(categoryOrder))
	for _, cat := range categoryOrder {
		inOrder[string(cat)] = true
	}
	for _, m := range catalog.DockerfileModules {
		if !inOrder[string(m.Category)] {
			t.Errorf("module %q has category %q missing from categoryOrder; it would be silently dropped", m.ID, m.Category)
		}
	}
}
