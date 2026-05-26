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
