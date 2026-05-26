package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

type ImageEntry struct {
	ProjectDir  string `json:"projectDir"`
	Workspace   string `json:"workspace"`
	Mode        string `json:"mode"`
	Image       string `json:"image"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Variant     string `json:"variant,omitempty"`
	CreatedAt   string `json:"createdAt"`
	LastUpdated string `json:"lastUpdated"`
}

type registryFile struct {
	Entries []ImageEntry `json:"entries"`
}

func ImageRegistryPath() string {
	return filepath.Join(GlobalConfigDir(), "images.json")
}

func LoadRegistry() []ImageEntry {
	data, err := os.ReadFile(ImageRegistryPath())
	if err != nil {
		return nil
	}
	var rf registryFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil
	}
	return rf.Entries
}

func SaveRegistry(entries []ImageEntry) error {
	dir := GlobalConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registryFile{Entries: entries}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(ImageRegistryPath(), data, 0644)
}

func RecordEntry(e ImageEntry) error {
	entries := LoadRegistry()
	found := false
	for i, existing := range entries {
		if existing.ProjectDir == e.ProjectDir {
			entries[i] = e
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, e)
	}
	return SaveRegistry(entries)
}

func RemoveEntry(projectDir string) bool {
	entries := LoadRegistry()
	for i, e := range entries {
		if e.ProjectDir == projectDir {
			entries = append(entries[:i], entries[i+1:]...)
			_ = SaveRegistry(entries)
			return true
		}
	}
	return false
}

func FindByFingerprint(fp string) *ImageEntry {
	for _, e := range LoadRegistry() {
		if e.Fingerprint == fp {
			copy := e
			return &copy
		}
	}
	return nil
}

func ListEntries() []ImageEntry {
	return LoadRegistry()
}

var labelLineRe = regexp.MustCompile(`(?i)^LABEL\b`)

func stripLabelBlocks(dockerfile string) string {
	lines := strings.Split(dockerfile, "\n")
	out := make([]string, 0, len(lines))
	inLabel := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inLabel {
			if labelLineRe.MatchString(trimmed) {
				inLabel = strings.HasSuffix(trimmed, "\\")
				continue
			}
			out = append(out, line)
		} else {
			inLabel = strings.HasSuffix(trimmed, "\\")
		}
	}
	return strings.Join(out, "\n")
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func ComputeFingerprint(dockerfileContent string, copyFileContents map[string]string, selectedModuleIDs []string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(stripLabelBlocks(dockerfileContent), "\r\n", "\n"))
	parts := []string{normalized}

	names := make([]string, 0, len(copyFileContents))
	for name := range copyFileContents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		parts = append(parts, filepath.Base(name)+"\x00"+sha256Hex(copyFileContents[name]))
	}

	sortedModuleIDs := append([]string{}, selectedModuleIDs...)
	sort.Strings(sortedModuleIDs)
	parts = append(parts, "MODULES\x00"+strings.Join(sortedModuleIDs, ","))

	return sha256Hex(strings.Join(parts, "\n"))
}

func FingerprintTag(fp string) string {
	if len(fp) < 12 {
		return "devcontainer-cli/" + fp + ":latest"
	}
	return "devcontainer-cli/" + fp[:12] + ":latest"
}

// LocalImageExists reports whether a Docker image with the given reference is
// present locally, using the injected capture func.
func LocalImageExists(image string, capture CaptureFunc) bool {
	status, stdout, _ := capture([]string{"images", "-q", image})
	return status == 0 && strings.TrimSpace(stdout) != ""
}

func RecordProject(projectDir string, config *core.DevcontainerConfig, customImage string) {
	now := time.Now().UTC().Format(time.RFC3339)
	img := customImage
	if img == "" {
		img = config.Image
	}
	variant := ""
	if config.Remote != nil {
		variant = config.Remote.Variant
	}
	_ = RecordEntry(ImageEntry{
		ProjectDir:  projectDir,
		Workspace:   config.Workspace,
		Mode:        string(config.Mode),
		Image:       img,
		Variant:     variant,
		Fingerprint: config.Fingerprint,
		CreatedAt:   now,
		LastUpdated: now,
	})
}
