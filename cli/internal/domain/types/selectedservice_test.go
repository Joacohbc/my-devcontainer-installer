package types

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestSelectedServiceUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name, in    string
		wantID      ServiceID
		wantVersion string
	}{
		{"bare string", `"mongo"`, "mongo", ""},
		{"object without options", `{"id":"redis"}`, "redis", ""},
		{"object with options", `{"id":"postgres","options":{"version":"16"}}`, "postgres", "16"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s SelectedService
			if err := json.Unmarshal([]byte(c.in), &s); err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if s.ID != c.wantID {
				t.Errorf("ID = %q, want %q", s.ID, c.wantID)
			}
			if s.Options == nil {
				t.Error("Options must never be nil after unmarshal")
			}
			if got, _ := s.Options["version"].(string); got != c.wantVersion {
				t.Errorf("version = %q, want %q", got, c.wantVersion)
			}
		})
	}
}

func TestSelectedServiceUnmarshalYAML(t *testing.T) {
	var bare SelectedService
	if err := yaml.Unmarshal([]byte("mongo\n"), &bare); err != nil {
		t.Fatalf("yaml bare: %v", err)
	}
	if bare.ID != "mongo" || bare.Options == nil {
		t.Errorf("bare yaml = %+v, want id=mongo non-nil options", bare)
	}

	var obj SelectedService
	if err := yaml.Unmarshal([]byte("id: postgres\noptions:\n  version: \"16\"\n"), &obj); err != nil {
		t.Fatalf("yaml object: %v", err)
	}
	if obj.ID != "postgres" || obj.Options["version"] != "16" {
		t.Errorf("object yaml = %+v, want id=postgres version=16", obj)
	}
}

// The tool always emits the object form (options omitted when empty); a
// no-options service must not round-trip into a bare string.
func TestSelectedServiceMarshalsToObjectForm(t *testing.T) {
	out, err := json.Marshal(SelectedService{ID: "mongo", Options: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"id":"mongo"}` {
		t.Errorf("marshal = %s, want {\"id\":\"mongo\"}", out)
	}
}

// A mixed services list (bare string + object) decodes into a fully typed slice.
func TestComposeConfigServicesMixedRoundTrip(t *testing.T) {
	const in = `{"services":["mongo",{"id":"postgres","options":{"version":"16"}}]}`
	var cfg ComposeConfig
	if err := json.Unmarshal([]byte(in), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []SelectedService{
		{ID: "mongo", Options: map[string]any{}},
		{ID: "postgres", Options: map[string]any{"version": "16"}},
	}
	if !reflect.DeepEqual(cfg.Services, want) {
		t.Errorf("services = %+v, want %+v", cfg.Services, want)
	}
}
