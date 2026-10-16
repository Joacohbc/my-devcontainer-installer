package domain

import (
	"encoding/binary"
	"encoding/json"
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

// ListUsedSubnets queries the engine (via the injected capture func) for the
// CIDR ranges the networks on the daemon occupy, except the ones named in ignoreNetworks. A project must ignore its
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
	status, stdout, _ := capture(append([]string{"network", "inspect"}, ids...))
	if status != 0 {
		return nil
	}
	var out []CidrRange
	for _, network := range parseNetworkInspect(stdout) {
		if ignored[network.Name] {
			continue
		}
		out = append(out, network.subnets()...)
	}
	return out
}

// inspectedNetwork is the part of `network inspect` this package reads. Docker
// lists a network's subnets under IPAM.Config and Podman under subnets; the
// JSON decoder matches field names case-insensitively, so the same type reads
// both spellings of the keys.
type inspectedNetwork struct {
	Name string
	IPAM struct {
		Config []struct{ Subnet string }
	}
	Subnets []struct{ Subnet string }
}

func parseNetworkInspect(stdout string) []inspectedNetwork {
	var networks []inspectedNetwork
	if err := json.Unmarshal([]byte(stdout), &networks); err != nil {
		return nil
	}
	return networks
}

func (n inspectedNetwork) subnets() []CidrRange {
	var cidrs []string
	for _, config := range n.IPAM.Config {
		cidrs = append(cidrs, config.Subnet)
	}
	for _, subnet := range n.Subnets {
		cidrs = append(cidrs, subnet.Subnet)
	}
	var ranges []CidrRange
	for _, cidr := range cidrs {
		if parsed, isValid := ParseCidr(cidr); isValid {
			ranges = append(ranges, *parsed)
		}
	}
	return ranges
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
