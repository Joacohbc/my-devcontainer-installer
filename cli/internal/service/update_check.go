package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

const (
	updateCheckTTL       = 24 * time.Hour
	updateRetryBackoff   = 1 * time.Hour
	updateCheckTimeoutMs = 1500
	maxSkillMDBytes      = 512 * 1024
	remoteSkillMDURL     = "https://raw.githubusercontent.com/" + selfUpdateRepo + "/main/skills/devcontainer-cli/SKILL.md"
	HostSkillInstallHint = "npx skills add " + selfUpdateRepo + "@devcontainer-cli -g"
)

var globalSkillRelativeDirs = []string{
	filepath.Join(".agents", "skills"),
	filepath.Join(".claude", "skills"),
	filepath.Join(".codex", "skills"),
	filepath.Join(".gemini", "skills"),
	filepath.Join(".gemini", "config", "skills"),
	filepath.Join(".gemini", "antigravity", "skills"),
	filepath.Join(".antigravity", "skills"),
	filepath.Join(".config", "opencode", "skills"),
}

var localSkillRelativeDirs = []string{
	filepath.Join(".agents", "skills"),
	filepath.Join(".claude", "skills"),
}

// UpdateCheckCache stores the last fetched CLI release tag and host skill digest.
type UpdateCheckCache struct {
	CheckedAt         time.Time `json:"checkedAt"`
	LatestCLITag      string    `json:"latestCliTag,omitempty"`
	RemoteSkillSHA256 string    `json:"remoteSkillSha256,omitempty"`
}

// SkillLocation describes one installed host copy of the devcontainer-cli skill.
type SkillLocation struct {
	Path        string
	DisplayPath string
	SHA256      string
}

// UpdateStatus reports whether the running CLI binary or any installed host skill copy is outdated.
type UpdateStatus struct {
	CurrentCLI    string
	LatestCLI     string
	CLIOutdated   bool
	SkillPaths    []string
	OutdatedPaths []string
	SkillChecked  bool
}

// UpdateCheckService validates CLI binary and host skill freshness using a 24h local cache.
type UpdateCheckService struct {
	Now              func() time.Time
	CachePath        func() string
	FetchLatestCLI   func() (string, error)
	FetchRemoteSkill func() ([]byte, error)
	FindSkills       func() []SkillLocation
	Report           Reporter
}

// Check returns the cached or freshly fetched update status for the CLI and installed host skills.
func (s UpdateCheckService) Check(currentCLI string, force bool) UpdateStatus {
	s = s.withDefaults()
	skills := s.FindSkills()
	cachePath := s.CachePath()
	cache := loadUpdateCheckCache(cachePath)
	now := s.Now()

	if needsCacheRefresh(cache, now, force) {
		cache = s.refreshCache(cachePath, cache, now)
	}
	return buildUpdateStatus(currentCLI, skills, cache)
}

// RecordCLITag updates the cached CLI tag after a self-update or explicit check.
func (s UpdateCheckService) RecordCLITag(tag string) {
	if tag == "" {
		return
	}
	s = s.withDefaults()
	cachePath := s.CachePath()
	cache := loadUpdateCheckCache(cachePath)
	cache.LatestCLITag = tag
	cache.CheckedAt = s.Now()
	_ = saveUpdateCheckCache(cachePath, cache)
}

// FormatUpdateWarnings converts outdated items in status into concise stderr warning lines.
func FormatUpdateWarnings(status UpdateStatus) []string {
	var warnings []string
	if status.CLIOutdated {
		warnings = append(warnings, fmt.Sprintf(
			"CLI update available: %s -> %s (run 'devcontainer-cli config upgrade-cli')",
			status.CurrentCLI, status.LatestCLI,
		))
	}
	if len(status.OutdatedPaths) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Host skill devcontainer-cli is outdated at %s (run '%s')",
			strings.Join(status.OutdatedPaths, ", "), HostSkillInstallHint,
		))
	}
	return warnings
}

func (s UpdateCheckService) withDefaults() UpdateCheckService {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.CachePath == nil {
		s.CachePath = domain.UpdateCheckCachePath
	}
	if s.FetchLatestCLI == nil {
		s.FetchLatestCLI = s.defaultFetchLatestCLI
	}
	if s.FetchRemoteSkill == nil {
		s.FetchRemoteSkill = defaultFetchRemoteSkill
	}
	if s.FindSkills == nil {
		s.FindSkills = FindInstalledHostSkills
	}
	return s
}

func (s UpdateCheckService) defaultFetchLatestCLI() (string, error) {
	svc := UpgradeService{Report: s.Report}
	top, stable, err := svc.UpgradeTargetsWithTimeout(updateCheckTimeoutMs)
	if err != nil {
		return "", err
	}
	target := svc.ResolveTarget(top, stable, false)
	if target == nil {
		return "", nil
	}
	return target.Tag, nil
}

func defaultFetchRemoteSkill() ([]byte, error) {
	status, body, err := httpGetWithTimeout(remoteSkillMDURL, false, maxSkillMDBytes, updateCheckTimeoutMs)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("skill check HTTP %d", status)
	}
	return body, nil
}

func (s UpdateCheckService) refreshCache(cachePath string, prev UpdateCheckCache, now time.Time) UpdateCheckCache {
	var cliTag string
	var skillBody []byte
	var cliErr, skillErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cliTag, cliErr = s.FetchLatestCLI()
	}()
	go func() {
		defer wg.Done()
		skillBody, skillErr = s.FetchRemoteSkill()
	}()
	wg.Wait()

	next := prev
	if cliErr == nil && cliTag != "" {
		next.LatestCLITag = cliTag
	}
	if skillErr == nil && len(skillBody) > 0 {
		next.RemoteSkillSHA256 = sha256NormalizedHex(skillBody)
	}

	next.CheckedAt = now
	if cliErr != nil && skillErr != nil {
		// Short backoff when offline so a transient network loss does not skip checking for a full day.
		next.CheckedAt = now.Add(-(updateCheckTTL - updateRetryBackoff))
	}
	_ = saveUpdateCheckCache(cachePath, next)
	return next
}

func needsCacheRefresh(cache UpdateCheckCache, now time.Time, force bool) bool {
	if force || cache.CheckedAt.IsZero() {
		return true
	}
	return now.Sub(cache.CheckedAt) >= updateCheckTTL
}

func buildUpdateStatus(currentCLI string, skills []SkillLocation, cache UpdateCheckCache) UpdateStatus {
	status := UpdateStatus{
		CurrentCLI:   currentCLI,
		LatestCLI:    cache.LatestCLITag,
		SkillChecked: cache.RemoteSkillSHA256 != "",
	}

	if isComparableCLIVersion(currentCLI) && cache.LatestCLITag != "" {
		status.CLIOutdated = compareVersions(currentCLI, cache.LatestCLITag) < 0
	}

	for _, sk := range skills {
		status.SkillPaths = append(status.SkillPaths, sk.DisplayPath)
		if status.SkillChecked && sk.SHA256 != cache.RemoteSkillSHA256 {
			status.OutdatedPaths = append(status.OutdatedPaths, sk.DisplayPath)
		}
	}
	return status
}

func isComparableCLIVersion(v string) bool {
	return v != "" && v != "dev" && v != "test"
}

func sha256NormalizedHex(data []byte) string {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func loadUpdateCheckCache(path string) UpdateCheckCache {
	if path == "" {
		return UpdateCheckCache{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UpdateCheckCache{}
	}
	var cache UpdateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return UpdateCheckCache{}
	}
	return cache
}

func saveUpdateCheckCache(path string, cache UpdateCheckCache) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

// FindInstalledHostSkills scans standard global and workspace skill directories
// for installed copies of devcontainer-cli/SKILL.md, deduplicating symlinks.
func FindInstalledHostSkills() []SkillLocation {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	candidates := skillCandidatePaths(home, cwd)

	seen := make(map[string]bool, len(candidates))
	var found []SkillLocation
	for _, path := range candidates {
		loc, canonical, ok := readSkillLocation(path, home)
		if !ok || seen[canonical] {
			continue
		}
		seen[canonical] = true
		found = append(found, loc)
	}
	return found
}

func skillCandidatePaths(home, cwd string) []string {
	var paths []string
	if home != "" {
		for _, rel := range globalSkillRelativeDirs {
			paths = append(paths, filepath.Join(home, rel, "devcontainer-cli", "SKILL.md"))
		}
	}
	if cwd != "" {
		for _, rel := range localSkillRelativeDirs {
			paths = append(paths, filepath.Join(cwd, rel, "devcontainer-cli", "SKILL.md"))
		}
	}
	return paths
}

func readSkillLocation(path, home string) (loc SkillLocation, canonical string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillLocation{}, "", false
	}
	canonical, err = filepath.EvalSymlinks(path)
	if err != nil {
		canonical = path
	}
	return SkillLocation{
		Path:        path,
		DisplayPath: displaySkillDir(path, home),
		SHA256:      sha256NormalizedHex(data),
	}, canonical, true
}

func displaySkillDir(skillMDPath, home string) string {
	dir := filepath.Dir(skillMDPath)
	if home == "" {
		return dir
	}
	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(dir, prefix) {
		return "~" + string(filepath.Separator) + strings.TrimPrefix(dir, prefix)
	}
	return dir
}
