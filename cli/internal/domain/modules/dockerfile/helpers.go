package dockerfile

import "strings"

// aptCleanupCommands is the single, standardized list of shell commands that
// purge apt caches and scratch directories after installing packages. It is the
// one source of truth shared by CleanupModule's final pass and by the inline
// cleanup appended to every apt install RUN, so the cleanup behaviour is
// identical across the whole Dockerfile.
var aptCleanupCommands = []string{
	"apt-get autoremove -y",
	"apt-get autoclean",
	"rm -rf /var/lib/apt/lists/*",
	"rm -rf /tmp/*",
	"rm -rf /var/tmp/*",
}

// aptCleanup renders the standardized cleanup as a single shell snippet (the
// commands joined by " && "), ready to chain onto the end of a RUN that installs
// apt packages so the cache never lands in that layer.
func aptCleanup() string {
	return strings.Join(aptCleanupCommands, " && ")
}

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
