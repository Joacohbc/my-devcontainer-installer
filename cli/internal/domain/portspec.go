package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// maxPort is the highest valid TCP/UDP port number.
const maxPort = 65535

// ValidatePortSpecs checks *published* port specs — the compose grammar — that
// did not come from a person typing them at a prompt, a profile's say, so a typo
// surfaces at generate time rather than as a compose error when the stack comes
// up.
//
// Both shapes a profile may use are accepted: a bare container port ("3000"),
// which leaves the host port to Docker, and an explicit mapping ("3000:3000" or
// "0.0.0.0:3000:3000"). A "/tcp" or "/udp" suffix is allowed on the last field.
//
// The tunnels of `port-forward` are a different grammar — their middle field is
// a compose service name, not a host IP, and they may carry a "reverse:"
// direction prefix — and are checked by the parser that opens them,
// parsePortMapping.
func ValidatePortSpecs(specs []string) error {
	for _, spec := range specs {
		if err := validatePortSpec(spec); err != nil {
			return err
		}
	}
	return nil
}

func validatePortSpec(spec string) error {
	if strings.TrimSpace(spec) == "" {
		return fmt.Errorf("empty port spec")
	}
	fields := strings.Split(spec, ":")
	if len(fields) > 3 {
		return fmt.Errorf("invalid port spec %q: expected PORT, HOST:CONTAINER or IP:HOST:CONTAINER", spec)
	}
	// Only the last field carries the protocol suffix, and in the three-field
	// form the first is a host IP rather than a port.
	portFields := fields
	if len(fields) == 3 {
		portFields = fields[1:]
	}
	for i, field := range portFields {
		isLast := i == len(portFields)-1
		if err := validatePortField(field, isLast, spec); err != nil {
			return err
		}
	}
	return nil
}

func validatePortField(field string, allowProtocol bool, spec string) error {
	// An empty host port is how compose says "any free one", so it is only
	// invalid when it is the container port.
	if field == "" && !allowProtocol {
		return nil
	}
	if allowProtocol {
		field, _, _ = strings.Cut(field, "/")
	}
	port, err := strconv.Atoi(field)
	if err != nil || port < 1 || port > maxPort {
		return fmt.Errorf("invalid port spec %q: %q is not a port between 1 and %d", spec, field, maxPort)
	}
	return nil
}
