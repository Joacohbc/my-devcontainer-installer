package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RunningForward records one detached SSH tunnel spawned for a project. The set
// of running forwards is persisted per-project (under .dc_<workspace>/) so the
// CLI can list and kill them across invocations.
type RunningForward struct {
	ID            string `json:"id"`
	PID           int    `json:"pid"`
	LocalPort     int    `json:"localPort"`
	ContainerPort int    `json:"containerPort"`
	TargetHost    string `json:"targetHost"`
	Alias         string `json:"alias"`
	StartedAt     string `json:"startedAt"`
}

// ForwardID is the unique, deterministic id of a forward. A local port can only
// be bound once, so "<workspace>-<localPort>" is naturally unique and lets a
// re-run of `up` detect an already-running tunnel instead of duplicating it.
func ForwardID(workspace string, localPort int) string {
	return fmt.Sprintf("%s-%d", workspace, localPort)
}

// LoadForwardState reads the per-project registry of running forwards. A missing
// file is not an error: it returns an empty slice.
func LoadForwardState(stateFile string) ([]RunningForward, error) {
	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var forwards []RunningForward
	if err := json.Unmarshal(data, &forwards); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filepath.Base(stateFile), err)
	}
	return forwards, nil
}

// SaveForwardState writes the registry, creating the project directory if
// needed. An empty registry removes the file so a clean project has no stray
// state.
func SaveForwardState(stateFile string, forwards []RunningForward) error {
	if len(forwards) == 0 {
		err := os.Remove(stateFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(forwards, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(stateFile, data, 0644)
}
