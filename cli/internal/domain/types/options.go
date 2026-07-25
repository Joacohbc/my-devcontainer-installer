package types

// CoerceStrings coerces a JSON/YAML-decoded value ([]string or []any of
// strings) into a []string, dropping non-string entries. It returns nil for any
// other type.
func CoerceStrings(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, x := range s {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

// StringOpt returns opts[key] as a non-empty string, or def when the value is
// absent, not a string, or empty.
func StringOpt(opts map[string]any, key, def string) string {
	if s, ok := opts[key].(string); ok && s != "" {
		return s
	}
	return def
}

// BoolOpt returns opts[key] as a bool, or def when the value is absent or not a
// bool.
func BoolOpt(opts map[string]any, key string, def bool) bool {
	if b, ok := opts[key].(bool); ok {
		return b
	}
	return def
}

// StringsOpt coerces opts[key] into a non-empty []string, falling back to def
// when the value is absent, the wrong type, or empty.
func StringsOpt(opts map[string]any, key string, def []string) []string {
	if out := CoerceStrings(opts[key]); len(out) > 0 {
		return out
	}
	return def
}
