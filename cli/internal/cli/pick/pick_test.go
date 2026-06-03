package pick

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

func TestContainerWorkspace(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"myws-" + sshdefaults.ServiceName, "myws"},
		{"my-cool-project-" + sshdefaults.ServiceName, "my-cool-project"},
		{"some-other-container", ""},
		{sshdefaults.ServiceName, ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ContainerWorkspace(c.in); got != c.want {
			t.Errorf("ContainerWorkspace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
