package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeRouteDomain(t *testing.T) {
	tests := []struct {
		name      string
		rawDomain string
		workspace string
		port      int
		want      string
	}{
		{
			name:      "empty domain uses workspace and port",
			rawDomain: "",
			workspace: "myproject",
			port:      3000,
			want:      "myproject-3000.devcli.localhost",
		},
		{
			name:      "empty domain with special characters in workspace sanitizes workspace",
			rawDomain: "",
			workspace: "my_special@project",
			port:      8080,
			want:      "my-special-project-8080.devcli.localhost",
		},
		{
			name:      "empty domain with empty workspace falls back to app",
			rawDomain: "",
			workspace: "",
			port:      5000,
			want:      "app-5000.devcli.localhost",
		},
		{
			name:      "short domain without dot appends devcli.localhost",
			rawDomain: "api",
			workspace: "myproject",
			port:      3000,
			want:      "api.devcli.localhost",
		},
		{
			name:      "short domain with uppercase and spaces normalizes cleanly",
			rawDomain: "  API-Gateway  ",
			workspace: "myproject",
			port:      3000,
			want:      "api-gateway.devcli.localhost",
		},
		{
			name:      "full domain with dots preserved as-is",
			rawDomain: "api.devcli.localhost",
			workspace: "myproject",
			port:      3000,
			want:      "api.devcli.localhost",
		},
		{
			name:      "custom local domain with dots preserved as-is",
			rawDomain: "custom.app.localhost",
			workspace: "myproject",
			port:      3000,
			want:      "custom.app.localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRouteDomain(tt.rawDomain, tt.workspace, tt.port)
			if got != tt.want {
				t.Errorf("NormalizeRouteDomain(%q, %q, %d) = %q, want %q", tt.rawDomain, tt.workspace, tt.port, got, tt.want)
			}
		})
	}
}

func TestBuildRouteID(t *testing.T) {
	id1 := BuildRouteID("myws", "api.devcli.localhost")
	id2 := BuildRouteID("myws", "api.devcli.localhost")
	id3 := BuildRouteID("otherws", "api.devcli.localhost")

	if id1 != id2 {
		t.Errorf("BuildRouteID should be deterministic, got %q != %q", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("BuildRouteID for different workspaces should differ, got %q == %q", id1, id3)
	}
	if !strings.HasPrefix(id1, "devcli_myws_") {
		t.Errorf("BuildRouteID should have prefix 'devcli_myws_', got %q", id1)
	}
}

func TestValidateRouteRule(t *testing.T) {
	validRule := RouteRule{
		Domain:     "api.devcli.localhost",
		TargetHost: "myproject-devcontainer-ssh",
		TargetPort: 3000,
		Workspace:  "myproject",
	}

	if err := ValidateRouteRule(validRule); err != nil {
		t.Fatalf("expected valid rule to pass validation, got %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(r *RouteRule)
		wantErr string
	}{
		{
			name: "invalid port 0",
			mutate: func(r *RouteRule) {
				r.TargetPort = 0
			},
			wantErr: "invalid target port",
		},
		{
			name: "invalid port > 65535",
			mutate: func(r *RouteRule) {
				r.TargetPort = 70000
			},
			wantErr: "invalid target port",
		},
		{
			name: "empty target host",
			mutate: func(r *RouteRule) {
				r.TargetHost = "   "
			},
			wantErr: "target host cannot be empty",
		},
		{
			name: "empty domain",
			mutate: func(r *RouteRule) {
				r.Domain = ""
			},
			wantErr: "domain cannot be empty",
		},
		{
			name: "invalid domain label with special chars",
			mutate: func(r *RouteRule) {
				r.Domain = "invalid_domain!.localhost"
			},
			wantErr: "invalid domain name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := validRule
			tt.mutate(&rule)
			err := ValidateRouteRule(rule)
			if err == nil {
				t.Fatalf("expected validation error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBuildCaddyRoute(t *testing.T) {
	rule := RouteRule{
		Domain:     "api.devcli.localhost",
		TargetHost: "myproject-devcontainer-ssh",
		TargetPort: 3000,
		Workspace:  "myproject",
	}

	caddyRoute, err := BuildCaddyRoute(rule)
	if err != nil {
		t.Fatalf("BuildCaddyRoute failed: %v", err)
	}

	if len(caddyRoute.Match) != 1 || len(caddyRoute.Match[0].Host) != 1 || caddyRoute.Match[0].Host[0] != "api.devcli.localhost" {
		t.Errorf("unexpected match hosts: %+v", caddyRoute.Match)
	}

	marshaled, err := json.Marshal(caddyRoute)
	if err != nil {
		t.Fatalf("marshalling caddy route: %v", err)
	}

	jsonStr := string(marshaled)
	if !strings.Contains(jsonStr, "myproject-devcontainer-ssh:3000") {
		t.Errorf("marshaled json missing upstream dial address: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "reverse_proxy") {
		t.Errorf("marshaled json missing reverse_proxy handler: %s", jsonStr)
	}
}

func TestBuildInitialCaddyConfig(t *testing.T) {
	cfg := BuildInitialCaddyConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(data)
	if !strings.Contains(str, ":80") || !strings.Contains(str, ":443") {
		t.Errorf("initial config missing listeners :80/:443: %s", str)
	}
}
