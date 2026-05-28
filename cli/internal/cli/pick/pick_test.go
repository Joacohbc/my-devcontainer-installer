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

func TestParsePSLines(t *testing.T) {
	stdout := `{"Names":"ws1-devcontainer-ssh","Image":"img1","Status":"Up 2 minutes","State":"running","Labels":"a=b","Ports":"22/tcp"}
{"Names":"ws2-devcontainer-ssh","Image":"img2","Status":"Exited (0)","State":"exited","Labels":"","Ports":""}

not-json-should-be-skipped
{"Image":"no-name-skipped","State":"running"}
`
	lines := parsePSLines(stdout)
	if len(lines) != 2 {
		t.Fatalf("expected 2 parsed lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Names != "ws1-devcontainer-ssh" || lines[0].Image != "img1" || lines[0].State != "running" {
		t.Errorf("line[0] mis-parsed: %+v", lines[0])
	}
	if lines[1].Names != "ws2-devcontainer-ssh" || lines[1].State != "exited" {
		t.Errorf("line[1] mis-parsed: %+v", lines[1])
	}
}

func TestParsePSLines_Empty(t *testing.T) {
	if lines := parsePSLines("   \n\n"); len(lines) != 0 {
		t.Errorf("expected no lines for blank input, got %+v", lines)
	}
}
