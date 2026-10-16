package domain_test

import (
	"reflect"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestParseLabelPairs(t *testing.T) {
	got := domain.ParseLabelPairs(" a=b, c=d=e ,,flag")
	want := map[string]string{"a": "b", "c": "d=e", "flag": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseLabelPairs = %v, want %v", got, want)
	}
	if got := domain.ParseLabelPairs(""); len(got) != 0 {
		t.Errorf("empty input must yield no labels, got %v", got)
	}
}

func TestParseLabelsJSON(t *testing.T) {
	want := map[string]string{"a": "b", "c": "d"}
	cases := map[string]string{
		"docker flattened": `"a=b,c=d"`,
		"podman object":    `{"a":"b","c":"d"}`,
	}
	for name, raw := range cases {
		if got := domain.ParseLabelsJSON([]byte(raw)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: ParseLabelsJSON(%s) = %v, want %v", name, raw, got, want)
		}
	}
	for _, raw := range []string{`null`, `{}`, `""`, `garbage`} {
		if got := domain.ParseLabelsJSON([]byte(raw)); got == nil || len(got) != 0 {
			t.Errorf("ParseLabelsJSON(%s) = %v, want an empty non-nil map", raw, got)
		}
	}
}

func TestFlattenLabels_isStableAndRoundTrips(t *testing.T) {
	labels := map[string]string{"z": "1", "a": "2", "m": "3"}
	flat := domain.FlattenLabels(labels)
	if flat != "a=2,m=3,z=1" {
		t.Errorf("FlattenLabels = %q, want sorted pairs", flat)
	}
	if got := domain.ParseLabelPairs(flat); !reflect.DeepEqual(got, labels) {
		t.Errorf("round trip = %v, want %v", got, labels)
	}
}

func TestParseNamesJSON(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{`"one"`, []string{"one"}},
		{`"one, two"`, []string{"one", "two"}},
		{`["one","two"]`, []string{"one", "two"}},
		{`[]`, []string{}},
		{`""`, nil},
		{`garbage`, nil},
	}
	for _, c := range cases {
		got := domain.ParseNamesJSON([]byte(c.raw))
		if len(got) != len(c.want) || (len(got) > 0 && !reflect.DeepEqual(got, c.want)) {
			t.Errorf("ParseNamesJSON(%s) = %#v, want %#v", c.raw, got, c.want)
		}
	}
}

func TestParsePortsJSON(t *testing.T) {
	cases := map[string]struct{ raw, want string }{
		"docker string": {`"127.0.0.1:8080->80/tcp, 2222/tcp"`, "127.0.0.1:8080->80/tcp, 2222/tcp"},
		"podman list": {
			`[{"host_ip":"127.0.0.1","container_port":22,"host_port":40222,"range":1,"protocol":"tcp"}]`,
			"127.0.0.1:40222->22/tcp",
		},
		"podman unpublished": {`[{"container_port":9000,"host_port":0}]`, "9000/tcp"},
		"podman several": {
			`[{"host_ip":"","container_port":80,"host_port":8080,"protocol":"tcp"},{"host_ip":"","container_port":53,"host_port":5353,"protocol":"udp"}]`,
			"0.0.0.0:8080->80/tcp, 0.0.0.0:5353->53/udp",
		},
		"null":    {`null`, ""},
		"garbage": {`{`, ""},
	}
	for name, c := range cases {
		if got := domain.ParsePortsJSON([]byte(c.raw)); got != c.want {
			t.Errorf("%s: ParsePortsJSON = %q, want %q", name, got, c.want)
		}
	}
}
