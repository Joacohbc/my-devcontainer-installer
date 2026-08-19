package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultDomainSuffix    = "devcli.localhost"
	RouterContainerName    = "devcli-router"
	RouterImage            = "caddy:alpine"
	RouterAdminPort        = 2019
	RouterHTTPPort         = 80
	RouterHTTPSPort        = 443
	RouterDataVolumeName   = "devcontainer-router-data"
	RouterConfigVolumeName = "devcontainer-router-config"
)

var validDomainLabelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type RouteRule struct {
	ID         string    `json:"id"`
	Domain     string    `json:"domain"`
	TargetHost string    `json:"target_host"`
	TargetPort int       `json:"target_port"`
	Workspace  string    `json:"workspace"`
	CreatedAt  time.Time `json:"created_at"`
}

type RouterState struct {
	Routes []RouteRule `json:"routes"`
}

type CaddyUpstream struct {
	Dial string `json:"dial"`
}

type CaddyReverseProxyHandler struct {
	Handler   string          `json:"handler"`
	Upstreams []CaddyUpstream `json:"upstreams"`
}

type CaddyRouteMatch struct {
	Host []string `json:"host"`
}

type CaddyRoute struct {
	ID     string                     `json:"@id,omitempty"`
	Match  []CaddyRouteMatch          `json:"match"`
	Handle []CaddyReverseProxyHandler `json:"handle"`
}

type CaddyHTTPServer struct {
	Listen []string     `json:"listen"`
	Routes []CaddyRoute `json:"routes"`
}

type CaddyHTTPApp struct {
	Servers map[string]CaddyHTTPServer `json:"servers"`
}

type CaddyApps struct {
	HTTP CaddyHTTPApp `json:"http"`
}

type CaddyAdmin struct {
	Listen        string   `json:"listen,omitempty"`
	EnforceOrigin bool     `json:"enforce_origin"`
	Origins       []string `json:"origins,omitempty"`
}

type CaddyConfig struct {
	Admin CaddyAdmin `json:"admin,omitempty"`
	Apps  CaddyApps  `json:"apps"`
}

func NormalizeRouteDomain(rawDomain, workspace string, port int) string {
	cleaned := strings.TrimSpace(strings.ToLower(rawDomain))
	if cleaned == "" {
		sanitizedWorkspace := sanitizeDomainLabel(workspace)
		if sanitizedWorkspace == "" {
			sanitizedWorkspace = "app"
		}
		return fmt.Sprintf("%s-%d.%s", sanitizedWorkspace, port, DefaultDomainSuffix)
	}

	if strings.Contains(cleaned, ".") {
		return cleaned
	}

	sanitized := sanitizeDomainLabel(cleaned)
	return fmt.Sprintf("%s.%s", sanitized, DefaultDomainSuffix)
}

func sanitizeDomainLabel(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	res := strings.Trim(b.String(), "-")
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	return res
}

func BuildRouteID(workspace, domain string) string {
	hasher := sha256.New()
	hasher.Write([]byte(workspace + ":" + domain))
	hashHex := hex.EncodeToString(hasher.Sum(nil))[:8]

	sanitizedWs := sanitizeDomainLabel(workspace)
	if sanitizedWs == "" {
		sanitizedWs = "ws"
	}
	return fmt.Sprintf("devcli_%s_%s", sanitizedWs, hashHex)
}

func ValidateRouteRule(rule RouteRule) error {
	if rule.TargetPort <= 0 || rule.TargetPort > 65535 {
		return fmt.Errorf("invalid target port %d: must be between 1 and 65535", rule.TargetPort)
	}

	if strings.TrimSpace(rule.TargetHost) == "" {
		return fmt.Errorf("target host cannot be empty")
	}

	domainClean := strings.TrimSpace(strings.ToLower(rule.Domain))
	if domainClean == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	labels := strings.Split(domainClean, ".")
	for _, l := range labels {
		if l == "" || !validDomainLabelRe.MatchString(l) {
			return fmt.Errorf("invalid domain name %q (label %q is invalid)", rule.Domain, l)
		}
	}

	return nil
}

func BuildCaddyRoute(rule RouteRule) (CaddyRoute, error) {
	if err := ValidateRouteRule(rule); err != nil {
		return CaddyRoute{}, err
	}

	id := rule.ID
	if id == "" {
		id = BuildRouteID(rule.Workspace, rule.Domain)
	}

	target := net.JoinHostPort(rule.TargetHost, fmt.Sprintf("%d", rule.TargetPort))
	handler := CaddyReverseProxyHandler{
		Handler: "reverse_proxy",
		Upstreams: []CaddyUpstream{
			{Dial: target},
		},
	}

	return CaddyRoute{
		ID: id,
		Match: []CaddyRouteMatch{
			{Host: []string{rule.Domain}},
		},
		Handle: []CaddyReverseProxyHandler{handler},
	}, nil
}

func BuildInitialCaddyConfig() CaddyConfig {
	return CaddyConfig{
		Admin: CaddyAdmin{
			Listen:  "0.0.0.0:2019",
			Origins: []string{"127.0.0.1:2019", "localhost:2019"},
		},
		Apps: CaddyApps{
			HTTP: CaddyHTTPApp{
				Servers: map[string]CaddyHTTPServer{
					"srv0": {
						Listen: []string{":80", ":443"},
						Routes: []CaddyRoute{},
					},
				},
			},
		},
	}
}
