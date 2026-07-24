package types

import (
	"reflect"
	"testing"
)

func TestStringOpt(t *testing.T) {
	opts := map[string]any{"present": "value", "empty": "", "wrong": 42}
	cases := []struct {
		name, key, def, want string
	}{
		{"present", "present", "d", "value"},
		{"empty falls back", "empty", "d", "d"},
		{"missing falls back", "missing", "d", "d"},
		{"wrong type falls back", "wrong", "d", "d"},
	}
	for _, c := range cases {
		if got := StringOpt(opts, c.key, c.def); got != c.want {
			t.Errorf("%s: StringOpt=%q, want %q", c.name, got, c.want)
		}
	}
}

func TestBoolOpt(t *testing.T) {
	opts := map[string]any{"yes": true, "no": false, "wrong": "true"}
	cases := []struct {
		name, key string
		def, want bool
	}{
		{"true value", "yes", false, true},
		{"false value", "no", true, false},
		{"missing uses def", "missing", true, true},
		{"wrong type uses def", "wrong", true, true},
	}
	for _, c := range cases {
		if got := BoolOpt(opts, c.key, c.def); got != c.want {
			t.Errorf("%s: BoolOpt=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestStringsOptAndCoerceStrings(t *testing.T) {
	def := []string{"d1", "d2"}
	opts := map[string]any{
		"strs":  []string{"a", "b"},
		"anys":  []any{"x", 1, "y"},
		"empty": []string{},
		"wrong": "nope",
	}
	cases := []struct {
		name, key string
		want      []string
	}{
		{"[]string passes through", "strs", []string{"a", "b"}},
		{"[]any keeps only strings", "anys", []string{"x", "y"}},
		{"empty slice falls back", "empty", def},
		{"missing falls back", "missing", def},
		{"wrong type falls back", "wrong", def},
	}
	for _, c := range cases {
		if got := StringsOpt(opts, c.key, def); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: StringsOpt=%v, want %v", c.name, got, c.want)
		}
	}

	if got := CoerceStrings(42); got != nil {
		t.Errorf("CoerceStrings(non-slice) = %v, want nil", got)
	}
}
