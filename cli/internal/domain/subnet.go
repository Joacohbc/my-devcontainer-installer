package domain

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
)

type CidrRange struct {
	Prefix netip.Prefix
}

// AddrToUint32 converts an IPv4 address to uint32.
func AddrToUint32(addr netip.Addr) uint32 {
	v4 := addr.As4()
	return binary.BigEndian.Uint32(v4[:])
}

// Uint32ToAddr converts a uint32 to an IPv4 netip.Addr.
func Uint32ToAddr(n uint32) netip.Addr {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], n)
	return netip.AddrFrom4(buf)
}

func (r CidrRange) Start() uint32 {
	return AddrToUint32(r.Prefix.Addr())
}

func (r CidrRange) End() uint32 {
	bits := r.Prefix.Bits()
	size := uint32(1) << (32 - bits)
	return r.Start() + size - 1
}

// ParseCidr parses a CIDR string and returns a CidrRange.
func ParseCidr(cidr string) (*CidrRange, bool) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil || !p.Addr().Is4() {
		return nil, false
	}
	return &CidrRange{Prefix: p.Masked()}, true
}

func RangesOverlap(a, b CidrRange) bool {
	return a.Prefix.Overlaps(b.Prefix)
}

func FindFreeSubnet(preferred string, usedSubnets []CidrRange) string {
	pref, ok := ParseCidr(preferred)
	if ok {
		overlaps := false
		for _, u := range usedSubnets {
			if RangesOverlap(*pref, u) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			return preferred
		}
	}
	for _, cand := range subnetCandidates() {
		if cand == preferred {
			continue
		}
		c, ok := ParseCidr(cand)
		if !ok {
			continue
		}
		overlaps := false
		for _, u := range usedSubnets {
			if RangesOverlap(*c, u) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			return cand
		}
	}
	return preferred
}

func SubnetConflict(cidr string, usedSubnets []CidrRange) *CidrRange {
	c, ok := ParseCidr(cidr)
	if !ok {
		return nil
	}
	for _, u := range usedSubnets {
		if RangesOverlap(*c, u) {
			return &u
		}
	}
	return nil
}

func FormatCidr(r CidrRange) string {
	return r.Prefix.String()
}

func NthHost(cidr string, n int) (string, bool) {
	r, ok := ParseCidr(cidr)
	if !ok {
		return "", false
	}
	start := r.Start()
	end := r.End()
	ip := start + uint32(n)
	if ip <= start || ip >= end {
		return "", false
	}
	return Uint32ToAddr(ip).String(), true
}

func LastHost(cidr string) (string, bool) {
	r, ok := ParseCidr(cidr)
	if !ok {
		return "", false
	}
	start := r.Start()
	end := r.End()
	ip := end - 1
	if ip <= start || ip >= end {
		return "", false
	}
	return Uint32ToAddr(ip).String(), true
}

// ListUsedSubnets queries Docker (via the injected capture func) for the CIDR
// ranges already allocated to existing networks on the daemon
// occupies, except the ones named in ignoreNetworks. A project must ignore its
// own network: regenerating a project whose stack is up would otherwise find
// its own subnet taken and move the project onto a different one on every run.
func ListUsedSubnets(capture CaptureFunc, ignoreNetworks ...string) []CidrRange {
	ignored := make(map[string]bool, len(ignoreNetworks))
	for _, name := range ignoreNetworks {
		if name != "" {
			ignored[name] = true
		}
	}
	return listUsedSubnets(capture, ignored)
}

func listUsedSubnets(capture CaptureFunc, ignored map[string]bool) []CidrRange {
	idStatus, idStdout, _ := capture([]string{"network", "ls", "--quiet"})
	if idStatus != 0 {
		return nil
	}
	var ids []string
	for _, line := range strings.Split(idStdout, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"network", "inspect", "--format", "{{$name := .Name}}{{range .IPAM.Config}}{{$name}} {{.Subnet}}\n{{end}}"}, ids...)
	status, stdout, _ := capture(args)
	if status != 0 {
		return nil
	}
	var out []CidrRange
	for _, line := range strings.Split(stdout, "\n") {
		name, cidr, ok := splitNetworkSubnet(line)
		if !ok || ignored[name] {
			continue
		}
		if c, parsed := ParseCidr(cidr); parsed {
			out = append(out, *c)
		}
	}
	return out
}

// splitNetworkSubnet parses one "<network> <cidr>" line. A line carrying only a
// CIDR is accepted with an empty name, so a network listing produced by an
// older format still counts as used rather than being silently dropped.
func splitNetworkSubnet(line string) (name, cidr string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	switch len(fields) {
	case 0:
		return "", "", false
	case 1:
		return "", fields[0], true
	default:
		return fields[0], fields[1], true
	}
}

func subnetCandidates() []string {
	var candidates []string
	for c := 0; c <= 255; c++ {
		for d := 0; d <= 240; d += 16 {
			candidates = append(candidates, fmt.Sprintf("172.25.%d.%d/28", c, d))
		}
	}
	for b := 26; b <= 254; b++ {
		candidates = append(candidates, fmt.Sprintf("172.%d.0.0/28", b))
	}
	for b := 0; b <= 254; b++ {
		candidates = append(candidates, fmt.Sprintf("10.%d.0.0/28", b))
	}
	return candidates
}
