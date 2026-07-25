package domain

import (
	"reflect"
	"testing"
)

func TestMapLocalNetwork(t *testing.T) {
	const networkName = "ws-network"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"local alias is rewritten", "local-network", networkName},
		{"other names pass through", "external-net", "external-net"},
		{"empty passes through", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapLocalNetwork(c.in, networkName); got != c.want {
				t.Errorf("mapLocalNetwork(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRemapServiceNetworks(t *testing.T) {
	const (
		networkName    = "ws-network"
		devcontainerIP = "172.25.0.254"
	)

	t.Run("non-slice value passes through unchanged", func(t *testing.T) {
		raw := map[string]any{"already": "mapped"}
		if got := remapServiceNetworks(raw, networkName, true, devcontainerIP); !reflect.DeepEqual(got, raw) {
			t.Errorf("expected passthrough, got %#v", got)
		}
	})

	t.Run("plain service maps the local alias to a slice", func(t *testing.T) {
		got := remapServiceNetworks([]string{"local-network", "extra"}, networkName, false, devcontainerIP)
		want := []string{networkName, "extra"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	})

	t.Run("devcontainer with an IP pins ipv4_address on its own network", func(t *testing.T) {
		got, ok := remapServiceNetworks([]string{"local-network", "extra"}, networkName, true, devcontainerIP).(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", got)
		}
		address, ok := got[networkName].(map[string]any)
		if !ok {
			t.Fatalf("expected ipv4 block on %q, got %#v", networkName, got[networkName])
		}
		if want := "${DEVCONTAINER_IP:-" + devcontainerIP + "}"; address["ipv4_address"] != want {
			t.Errorf("ipv4_address = %v, want %q", address["ipv4_address"], want)
		}
		if got["extra"] != nil {
			t.Errorf("non-primary network should have no address block, got %#v", got["extra"])
		}
	})

	t.Run("devcontainer without an IP stays a plain slice", func(t *testing.T) {
		got := remapServiceNetworks([]string{"local-network"}, networkName, true, "")
		if want := []string{networkName}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	})
}
