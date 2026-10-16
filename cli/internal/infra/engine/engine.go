package engine

import (
	"fmt"
	"strings"
	"sync"
)

type Engine struct {
	Name          string
	Binary        string
	RemoteHostVar string
}

var (
	EngineDocker = Engine{Name: "docker", Binary: "docker", RemoteHostVar: "DOCKER_HOST"}
	EnginePodman = Engine{Name: "podman", Binary: "podman", RemoteHostVar: "CONTAINER_HOST"}
)

const (
	PreferenceAuto   = "auto"
	PreferenceDocker = "docker"
	PreferencePodman = "podman"
)

var enginePreferences = []string{PreferenceAuto, PreferenceDocker, PreferencePodman}

func EnginePreferences() []string {
	return append([]string(nil), enginePreferences...)
}

func ParseEnginePreference(raw string) (string, error) {
	preference := strings.ToLower(strings.TrimSpace(raw))
	if preference == "" {
		return PreferenceAuto, nil
	}
	for _, known := range enginePreferences {
		if preference == known {
			return preference, nil
		}
	}
	return "", fmt.Errorf("invalid engine %q: expected one of %s", raw, strings.Join(enginePreferences, ", "))
}

var (
	engineMu         sync.Mutex
	enginePreference = PreferenceAuto
	resolvedEngine   *Engine
)

func SetEnginePreference(preference string) {
	engineMu.Lock()
	defer engineMu.Unlock()
	enginePreference = preference
	resolvedEngine = nil
	ResetAvailabilityCache()
}

func SetEngine(engine Engine) {
	engineMu.Lock()
	defer engineMu.Unlock()
	resolvedEngine = &engine
	ResetAvailabilityCache()
}

func ResetEngine() {
	engineMu.Lock()
	defer engineMu.Unlock()
	enginePreference = PreferenceAuto
	resolvedEngine = nil
	ResetAvailabilityCache()
}

func CurrentEngine() Engine {
	engineMu.Lock()
	defer engineMu.Unlock()
	if resolvedEngine != nil {
		return *resolvedEngine
	}
	engine, isFinal := resolveEngine(enginePreference)
	if isFinal {
		resolvedEngine = &engine
	}
	return engine
}

func resolveEngine(preference string) (engine Engine, isFinal bool) {
	switch preference {
	case PreferenceDocker:
		return EngineDocker, true
	case PreferencePodman:
		return EnginePodman, true
	}
	return autodetectEngine()
}

func autodetectEngine() (engine Engine, isFinal bool) {
	ctx := getContext()
	if ctx.Err() != nil {
		return EngineDocker, false
	}
	for _, candidate := range []Engine{EngineDocker, EnginePodman} {
		if !probeEngine(ctx, candidate) {
			continue
		}
		rememberAvailability(true)
		return candidate, true
	}
	rememberAvailability(false)
	return EngineDocker, ctx.Err() == nil
}

func rememberAvailability(isAvailable bool) {
	availMu.Lock()
	defer availMu.Unlock()
	availCache = &isAvailable
}

func unavailableError(engine Engine) error {
	if engine.Name == EngineDocker.Name && enginePreferenceIsAuto() {
		return fmt.Errorf("docker is not installed or the daemon is not running (podman is not available either)")
	}
	return fmt.Errorf("%s is not installed or the daemon is not running", engine.Name)
}

func enginePreferenceIsAuto() bool {
	engineMu.Lock()
	defer engineMu.Unlock()
	return enginePreference == PreferenceAuto
}

func IsPodman() bool {
	return CurrentEngine().Name == EnginePodman.Name
}

const (
	podmanRootlessFormat = "{{.Host.Security.Rootless}}"
	podmanSocketFormat   = "{{.Host.RemoteSocket.Path}}"
)

func IsRootless() bool {
	return podmanInfo(podmanRootlessFormat) == "true"
}

func HostSocketPath() string {
	return strings.TrimPrefix(podmanInfo(podmanSocketFormat), "unix://")
}

func podmanInfo(format string) string {
	if !IsPodman() {
		return ""
	}
	status, stdout, _, err := Capture([]string{"info", "--format", format})
	if err != nil || status != 0 {
		return ""
	}
	return strings.TrimSpace(stdout)
}
