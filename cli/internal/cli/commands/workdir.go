package commands

import (
	"fmt"
	"os"
)

// currentDir returns the process working directory, wrapping the rare
// os.Getwd failure with context so callers can surface it instead of
// silently proceeding with an empty path.
func currentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine current directory: %w", err)
	}
	return dir, nil
}
