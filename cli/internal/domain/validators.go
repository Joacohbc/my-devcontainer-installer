package domain

import (
	"net/netip"
	"regexp"
	"strings"
)

var (
	imageNameRe  = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*)*(?::[\w][\w.\-]{0,127})?$`)
	dockerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,62}$`)
	invalidRune  = regexp.MustCompile(`[^a-z0-9_.\-]+`)
	leadingBad   = regexp.MustCompile(`^[^a-z0-9]+`)
	trailingBad  = regexp.MustCompile(`[^a-z0-9]+$`)
)

func IsValidDockerName(name string) bool {
	return dockerNameRe.MatchString(name)
}

func SanitizeDockerName(raw string, fallback string) string {
	if fallback == "" {
		fallback = "devcontainer"
	}
	cleaned := strings.ToLower(raw)
	cleaned = invalidRune.ReplaceAllString(cleaned, "-")
	cleaned = leadingBad.ReplaceAllString(cleaned, "")
	cleaned = trailingBad.ReplaceAllString(cleaned, "")
	if len(cleaned) > 63 {
		cleaned = cleaned[:63]
	}
	if cleaned == "" {
		return fallback
	}
	return cleaned
}

func IsValidImageName(name string) bool {
	return imageNameRe.MatchString(name)
}

func IsValidCidr(cidr string) bool {
	p, err := netip.ParsePrefix(cidr)
	return err == nil && p.Addr().Is4()
}
