package domain_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestParseCidr_Valid(t *testing.T) {
	r, ok := domain.ParseCidr("172.25.0.0/28")
	if !ok {
		t.Fatal("expected ParseCidr to succeed for 172.25.0.0/28")
	}
	if r == nil {
		t.Fatal("expected non-nil CidrRange")
	}
}

func TestParseCidr_Invalid(t *testing.T) {
	_, ok := domain.ParseCidr("not-a-cidr")
	if ok {
		t.Error("expected ParseCidr to fail for invalid input")
	}
	_, ok = domain.ParseCidr("2001:db8::/32")
	if ok {
		t.Error("expected ParseCidr to fail for IPv6 CIDR")
	}
}

func TestLastHost_Slash28(t *testing.T) {
	ip, ok := domain.LastHost("172.25.0.0/28")
	if !ok {
		t.Fatal("expected LastHost to succeed")
	}
	if ip != "172.25.0.14" {
		t.Errorf("expected 172.25.0.14, got %q", ip)
	}
}

func TestLastHost_Slash24(t *testing.T) {
	ip, ok := domain.LastHost("172.26.0.0/24")
	if !ok {
		t.Fatal("expected LastHost to succeed")
	}
	if ip != "172.26.0.254" {
		t.Errorf("expected 172.26.0.254, got %q", ip)
	}
}

func TestNthHost(t *testing.T) {
	ip, ok := domain.NthHost("172.25.0.0/28", 1)
	if !ok {
		t.Fatal("expected NthHost to succeed")
	}
	if ip != "172.25.0.1" {
		t.Errorf("expected 172.25.0.1, got %q", ip)
	}
}

func TestNthHost_OutOfRange(t *testing.T) {
	_, ok := domain.NthHost("172.25.0.0/28", 0)
	if ok {
		t.Error("expected NthHost(n=0) to fail — it's the network address")
	}
}

func TestRangesOverlap_True(t *testing.T) {
	a, _ := domain.ParseCidr("172.25.0.0/28")
	b, _ := domain.ParseCidr("172.25.0.8/29")
	if !domain.RangesOverlap(*a, *b) {
		t.Error("expected overlapping ranges to report overlap")
	}
}

func TestRangesOverlap_False(t *testing.T) {
	a, _ := domain.ParseCidr("172.25.0.0/28")
	b, _ := domain.ParseCidr("172.25.0.16/28")
	if domain.RangesOverlap(*a, *b) {
		t.Error("expected non-overlapping ranges to not report overlap")
	}
}

func TestFindFreeSubnet_PreferredFree(t *testing.T) {
	preferred := "172.25.0.0/28"
	got := domain.FindFreeSubnet(preferred, nil)
	if got != preferred {
		t.Errorf("expected preferred subnet %q when no conflicts, got %q", preferred, got)
	}
}

func TestFindFreeSubnet_ConflictingPreferred(t *testing.T) {
	preferred := "172.25.0.0/28"
	used, _ := domain.ParseCidr(preferred)
	got := domain.FindFreeSubnet(preferred, []domain.CidrRange{*used})
	if got == preferred {
		t.Errorf("expected a different subnet when preferred is used, got %q", got)
	}
}

func TestFormatCidr(t *testing.T) {
	r, ok := domain.ParseCidr("172.25.0.0/28")
	if !ok {
		t.Fatal("parse failed")
	}
	got := domain.FormatCidr(*r)
	if got != "172.25.0.0/28" {
		t.Errorf("expected 172.25.0.0/28, got %q", got)
	}
}

// Docker prints a network's ranges under IPAM.Config; Podman prints them under
// subnets, with lower-case keys. Both must count as used.
const (
	dockerNetworks = `[{"Name":"bridge","IPAM":{"Config":[{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]}},
		{"Name":"api-network","IPAM":{"Config":[{"Subnet":"172.25.0.0/28"}]}}]`
	podmanNetworks = `[{"name":"podman","subnets":[{"subnet":"10.88.0.0/16","gateway":"10.88.0.1"}]},
		{"name":"api-network","subnets":[{"subnet":"172.25.0.0/28"}]}]`
)

// fakeNetworkCapture replies to the two calls ListUsedSubnets makes: the id
// listing and the unformatted inspect of those ids.
func fakeNetworkCapture(inspectOutput string) domain.CaptureFunc {
	return func(args []string) (int, string, string) {
		if len(args) > 1 && args[1] == "ls" {
			return 0, "id1\nid2\n", ""
		}
		return 0, inspectOutput, ""
	}
}

func TestListUsedSubnets(t *testing.T) {
	for name, inspect := range map[string]string{"docker": dockerNetworks, "podman": podmanNetworks} {
		used := domain.ListUsedSubnets(fakeNetworkCapture(inspect))
		if len(used) != 2 {
			t.Errorf("%s: expected both networks to count as used, got %v", name, used)
		}
	}
}

// A project's own network holds exactly the subnet the project is about to ask
// for, so counting it would move the project off its own range on every
// regeneration.
func TestListUsedSubnetsIgnoresNamedNetworks(t *testing.T) {
	cases := map[string]struct{ inspect, other string }{
		"docker": {dockerNetworks, "172.17.0.0/16"},
		"podman": {podmanNetworks, "10.88.0.0/16"},
	}
	for name, c := range cases {
		used := domain.ListUsedSubnets(fakeNetworkCapture(c.inspect), "api-network")
		if len(used) != 1 {
			t.Fatalf("%s: expected the ignored network to be excluded, got %v", name, used)
		}
		if domain.FormatCidr(used[0]) != c.other {
			t.Errorf("%s: expected the other network to survive, got %s", name, domain.FormatCidr(used[0]))
		}
		if domain.SubnetConflict("172.25.0.0/28", used) != nil {
			t.Errorf("%s: the project's own subnet must not conflict with itself", name)
		}
	}
}

func TestListUsedSubnetsToleratesUnreadableOutput(t *testing.T) {
	if used := domain.ListUsedSubnets(fakeNetworkCapture("not json")); len(used) != 0 {
		t.Errorf("unreadable inspect output must yield no ranges, got %v", used)
	}
}

func TestWorkspaceNetworkName(t *testing.T) {
	if got := domain.WorkspaceNetworkName("api"); got != "api-network" {
		t.Errorf("WorkspaceNetworkName = %q, want %q", got, "api-network")
	}
}
