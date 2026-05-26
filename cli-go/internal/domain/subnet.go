package domain

import (
	"fmt"
	"strings"
)

type CidrRange struct {
	Start uint32
	End   uint32
	Mask  uint32
}

func ParseCidr(cidr string) (*CidrRange, bool) {
	if !IsValidCidr(cidr) {
		return nil, false
	}
	parts := strings.SplitN(cidr, "/", 2)
	var mask uint32
	fmt.Sscanf(parts[1], "%d", &mask)
	octets := strings.Split(parts[0], ".")
	var ip uint32
	for _, o := range octets {
		var n uint32
		fmt.Sscanf(o, "%d", &n)
		ip = (ip << 8) | n
	}
	var maskBits uint32
	if mask == 0 {
		maskBits = 0
	} else {
		maskBits = ^uint32(0) << (32 - mask)
	}
	start := ip & maskBits
	end := start | (^maskBits)
	return &CidrRange{Start: start, End: end, Mask: mask}, true
}

func toIP(n uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(n>>24)&0xff,
		(n>>16)&0xff,
		(n>>8)&0xff,
		n&0xff,
	)
}

func RangesOverlap(a, b CidrRange) bool {
	return a.Start <= b.End && b.Start <= a.End
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
	return fmt.Sprintf("%s/%d", toIP(r.Start), r.Mask)
}

func NthHost(cidr string, n int) (string, bool) {
	r, ok := ParseCidr(cidr)
	if !ok {
		return "", false
	}
	ip := r.Start + uint32(n)
	if ip <= r.Start || ip >= r.End {
		return "", false
	}
	return toIP(ip), true
}

func LastHost(cidr string) (string, bool) {
	r, ok := ParseCidr(cidr)
	if !ok {
		return "", false
	}
	ip := r.End - 1
	if ip <= r.Start || ip >= r.End {
		return "", false
	}
	return toIP(ip), true
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
