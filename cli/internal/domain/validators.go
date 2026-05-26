package domain

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	imageNameRe  = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|[-]+)[a-z0-9]+)*)*(?::[\w][\w.\-]{0,127})?$`)
	cidrRe       = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`)
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
	if !cidrRe.MatchString(cidr) {
		return false
	}
	parts := strings.SplitN(cidr, "/", 2)
	mask, err := strconv.Atoi(parts[1])
	if err != nil || mask < 0 || mask > 32 {
		return false
	}
	octets := strings.Split(parts[0], ".")
	for _, octet := range octets {
		n, err := strconv.Atoi(octet)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}
