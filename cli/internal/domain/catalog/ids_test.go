package catalog

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestModuleIDsMatchCatalog(t *testing.T) {
	ids := ModuleIDs()
	if len(ids) != len(DockerfileModules) {
		t.Fatalf("ModuleIDs len=%d, want %d", len(ids), len(DockerfileModules))
	}
	for i, m := range DockerfileModules {
		if ids[i] != string(m.ID) {
			t.Errorf("ModuleIDs[%d]=%q, want %q", i, ids[i], m.ID)
		}
		if GetDockerfileModule(types.ModuleID(ids[i])) == nil {
			t.Errorf("ModuleIDs[%d]=%q is not resolvable", i, ids[i])
		}
	}
}

func TestServiceIDsMatchCatalog(t *testing.T) {
	ids := ServiceIDs()
	if len(ids) != len(ComposeServices) {
		t.Fatalf("ServiceIDs len=%d, want %d", len(ids), len(ComposeServices))
	}
	for i, s := range ComposeServices {
		if ids[i] != string(s.ID) {
			t.Errorf("ServiceIDs[%d]=%q, want %q", i, ids[i], s.ID)
		}
		if GetComposeService(types.ServiceID(ids[i])) == nil {
			t.Errorf("ServiceIDs[%d]=%q is not resolvable", i, ids[i])
		}
	}
}
