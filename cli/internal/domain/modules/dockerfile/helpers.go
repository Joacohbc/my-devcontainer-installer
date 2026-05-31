package dockerfile

func stringsFromAny(v any, def []string) []string {
	if s, ok := v.([]string); ok {
		if len(s) == 0 {
			return def
		}
		return s
	}
	arr, ok := v.([]any)
	if !ok {
		return def
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return def
	}
	return result
}
