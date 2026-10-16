package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Docker and Podman print the same fields of `ps`, `network ls` and friends in
// different shapes: Docker flattens labels, networks and ports into strings
// ("k=v,k=v"), Podman keeps them structured. The readers below accept both, so
// a command does not need to know which engine answered.

// ParseLabelPairs reads Docker's flattened "k=v,k=v" label string.
func ParseLabelPairs(flattened string) map[string]string {
	labels := map[string]string{}
	for _, pair := range strings.Split(flattened, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, _ := strings.Cut(pair, "=")
		labels[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return labels
}

// ParseLabelsJSON reads labels as `--format '{{json .Labels}}'` prints them:
// a JSON string of "k=v,k=v" pairs, a JSON object, or null.
func ParseLabelsJSON(raw []byte) map[string]string {
	var structured map[string]string
	if err := json.Unmarshal(raw, &structured); err == nil && structured != nil {
		return structured
	}
	var flattened string
	if err := json.Unmarshal(raw, &flattened); err == nil {
		return ParseLabelPairs(flattened)
	}
	return map[string]string{}
}

// FlattenLabels is the inverse of ParseLabelPairs, in a stable order.
func FlattenLabels(labels map[string]string) string {
	pairs := make([]string, 0, len(labels))
	for key, value := range labels {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

// ParseNamesJSON reads a name list as `{{json .Names}}` or `{{json .Networks}}`
// prints it: a JSON string (comma-separated when there are several) or an
// array of strings.
func ParseNamesJSON(raw []byte) []string {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var flattened string
	if err := json.Unmarshal(raw, &flattened); err != nil {
		return nil
	}
	var names []string
	for _, name := range strings.Split(flattened, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

type structuredPortBinding struct {
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// ParsePortsJSON reads published ports as `ps` prints them in JSON: Docker's
// ready-made string, or Podman's list of bindings, rendered the way Docker
// writes them ("127.0.0.1:8080->80/tcp").
func ParsePortsJSON(raw []byte) string {
	var bindings []structuredPortBinding
	if err := json.Unmarshal(raw, &bindings); err == nil {
		rendered := make([]string, 0, len(bindings))
		for _, binding := range bindings {
			rendered = append(rendered, renderPortBinding(binding))
		}
		return strings.Join(rendered, ", ")
	}
	var flattened string
	if err := json.Unmarshal(raw, &flattened); err == nil {
		return flattened
	}
	return ""
}

func renderPortBinding(binding structuredPortBinding) string {
	protocol := binding.Protocol
	if protocol == "" {
		protocol = "tcp"
	}
	container := fmt.Sprintf("%d/%s", binding.ContainerPort, protocol)
	if binding.HostPort == 0 {
		return container
	}
	hostIP := binding.HostIP
	if hostIP == "" {
		hostIP = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d->%s", hostIP, binding.HostPort, container)
}
