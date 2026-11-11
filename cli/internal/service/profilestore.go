package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
)

const legacyPresetsDirName = "presets"

// ProfileStore owns the user profiles directory: every profile is a directory
// <Dir>/<id>/ holding profile.yml and its scripts, and every change is recorded
// as one commit in the configuration repository.
type ProfileStore struct {
	Report Reporter
	Repo   ConfigRepo
	Dir    string
}

// NewProfileStore returns the store for the real profiles directory.
func NewProfileStore(report Reporter) ProfileStore {
	return ProfileStore{Report: report, Repo: NewConfigRepo(report), Dir: domain.ProfileDir()}
}

// Save writes p as <Dir>/<id>/profile.yml, copies each script that has a Source
// next to it, commits the result with message, and returns the manifest path.
func (s ProfileStore) Save(p catalog.Profile, message string) (string, error) {
	versioned := s.Repo.Prepare()

	profileDir := filepath.Join(s.Dir, p.ID)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create profile directory: %w", err)
	}
	for _, sc := range p.Scripts {
		if sc.Source == "" {
			continue
		}
		data, err := domain.ReadCustomScript(sc)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(profileDir, sc.File), data, 0o755); err != nil {
			return "", fmt.Errorf("failed to copy script %s: %w", sc.File, err)
		}
	}
	manifest, err := yaml.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to marshal profile: %w", err)
	}
	path := filepath.Join(profileDir, catalog.ProfileFileName)
	if err := os.WriteFile(path, manifest, 0o644); err != nil {
		return "", fmt.Errorf("failed to save profile: %w", err)
	}

	if versioned {
		s.Repo.Commit(message, profileDir)
	}
	return path, nil
}

// Locate validates that every id is an existing user profile, so a command can
// fail before it prompts. A built-in id and an unknown id are told apart.
func (s ProfileStore) Locate(ids []string) error {
	_, err := s.locate(ids)
	return err
}

func (s ProfileStore) locate(ids []string) ([]string, error) {
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		p, ok := catalog.Resolve(id, s.Dir)
		if !ok {
			return nil, fmt.Errorf("user profile %q not found", id)
		}
		if p.Source != "user" {
			return nil, fmt.Errorf("profile %q is a built-in profile and cannot be removed", id)
		}
		paths = append(paths, p.Dir)
	}
	return paths, nil
}

// Remove deletes the given user profiles and records one commit for the lot.
// Every id is validated before anything is deleted.
func (s ProfileStore) Remove(ids []string) error {
	paths, err := s.locate(ids)
	if err != nil {
		return err
	}
	versioned := s.Repo.Prepare()
	for i, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove profile %q: %w", ids[i], err)
		}
	}
	if versioned {
		s.Repo.Commit("chore(profile): remove "+strings.Join(ids, ", "), paths...)
	}
	return nil
}

// LegacyWarnings warns about on-disk layouts that are no longer read: flat
// profile files and the old presets/ directory.
func (s ProfileStore) LegacyWarnings() {
	for _, path := range catalog.LegacyFlatProfiles(s.Dir) {
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		s.Report.Warn("Ignoring flat profile file %s: profiles are directories now. Move it to %s.", path, filepath.Join(s.Dir, id, catalog.ProfileFileName))
	}
	presets := filepath.Join(filepath.Dir(s.Dir), legacyPresetsDirName)
	if info, err := os.Stat(presets); err == nil && info.IsDir() {
		s.Report.Warn("Ignoring legacy directory %s: it is no longer read. Move each profile into %s/<id>/profile.yml.", presets, s.Dir)
	}
}
