package domain

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestIsGitRepoURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://github.com/user/repo.git", true},
		{"http://gitlab.com/user/repo", true},
		{"git@github.com:user/repo.git", true},
		{"ssh://git@github.com/user/repo", true},
		{"https://bitbucket.org/user/repo", true},
		{"git://github.com/user/repo", true},
		{"my-local-dir", false},
		{"./relative/path", false},
		{"/absolute/path", false},
		{"", false},
	}

	for _, tt := range tests {
		result := IsGitRepoURL(tt.url)
		if result != tt.expected {
			t.Errorf("IsGitRepoURL(%q) = %v, expected %v", tt.url, result, tt.expected)
		}
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/my-app.git", "my-app"},
		{"https://github.com/user/my-app/", "my-app"},
		{"https://github.com/user/my-app", "my-app"},
		{"git@github.com:user/backend-service.git", "backend-service"},
		{"git@github.com:user/backend-service", "backend-service"},
		{"repo-name", "repo-name"},
	}

	for _, tt := range tests {
		result := ExtractRepoName(tt.url)
		if result != tt.expected {
			t.Errorf("ExtractRepoName(%q) = %q, expected %q", tt.url, result, tt.expected)
		}
	}
}

func TestDetectStack(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		dirs          []string
		wantModules   []types.ModuleID
		wantSkills    bool
		wantFilesSeen []string
	}{
		{
			name: "Node and pnpm project",
			files: map[string]string{
				"package.json":     `{"name": "test", "packageManager": "pnpm@9.0.0"}`,
				"tsconfig.json":    `{}`,
				"pnpm-lock.yaml":   ``,
				"skills-lock.json": `{}`,
			},
			wantModules:   []types.ModuleID{types.ModuleNodejs, types.ModulePnpm},
			wantSkills:    true,
			wantFilesSeen: []string{"package.json", "pnpm-lock.yaml", "skills-lock.json", "tsconfig.json"},
		},
		{
			name: "Go project",
			files: map[string]string{
				"go.mod":  "module example.com/test",
				"main.go": "package main",
			},
			wantModules: []types.ModuleID{types.ModuleGolang},
			wantSkills:  false,
		},
		{
			name: "Python uv project",
			files: map[string]string{
				"pyproject.toml": "[project]",
				"uv.lock":        "",
			},
			wantModules: []types.ModuleID{types.ModulePython},
			wantSkills:  false,
		},
		{
			name: "Java Maven project",
			files: map[string]string{
				"pom.xml": "<project></project>",
				"mvnw":    "#!/bin/sh",
			},
			wantModules: []types.ModuleID{types.ModuleJavaTemurin},
			wantSkills:  false,
		},
		{
			name: "Java Gradle project",
			files: map[string]string{
				"build.gradle.kts": "",
				"gradlew":          "#!/bin/sh",
			},
			wantModules: []types.ModuleID{types.ModuleJavaTemurin},
			wantSkills:  false,
		},
		{
			name: "PHP Composer project",
			files: map[string]string{
				"composer.json": "{}",
			},
			wantModules: []types.ModuleID{types.ModulePhp},
			wantSkills:  false,
		},
		{
			name: "Rust project",
			files: map[string]string{
				"Cargo.toml": "[package]",
			},
			wantModules: []types.ModuleID{types.ModuleRust},
			wantSkills:  false,
		},
		{
			name: "C++ CMake project",
			files: map[string]string{
				"CMakeLists.txt": "cmake_minimum_required(VERSION 3.10)",
			},
			wantModules: []types.ModuleID{types.ModuleCCpp},
			wantSkills:  false,
		},
		{
			name: "GitHub actions and Docker",
			dirs: []string{".github"},
			files: map[string]string{
				"Dockerfile": "FROM alpine",
			},
			wantModules: []types.ModuleID{types.ModuleGithubCli, types.ModuleDod},
			wantSkills:  false,
		},
		{
			name: "Nested agents skills lock",
			dirs: []string{".agents"},
			files: map[string]string{
				filepath.Join(".agents", "skills-lock.json"): `{}`,
			},
			wantModules: []types.ModuleID{types.ModuleNodejs},
			wantSkills:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, d := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
					t.Fatalf("MkdirAll failed: %v", err)
				}
			}
			for path, content := range tt.files {
				fullPath := filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
					t.Fatalf("MkdirAll for file failed: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
					t.Fatalf("WriteFile failed: %v", err)
				}
			}

			stack, err := DetectStack(dir)
			if err != nil {
				t.Fatalf("DetectStack failed: %v", err)
			}

			if stack.HasSkillsLock != tt.wantSkills {
				t.Errorf("HasSkillsLock = %v, expected %v", stack.HasSkillsLock, tt.wantSkills)
			}

			for _, mod := range tt.wantModules {
				if !slices.Contains(stack.Modules, mod) {
					t.Errorf("expected module %s in detected modules %v", mod, stack.Modules)
				}
			}

			for _, file := range tt.wantFilesSeen {
				if !slices.Contains(stack.DetectedFiles, file) {
					t.Errorf("expected detected file %s in %v", file, stack.DetectedFiles)
				}
			}
		})
	}
}
