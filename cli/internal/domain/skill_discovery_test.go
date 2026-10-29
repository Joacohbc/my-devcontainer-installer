package domain

import (
	"fmt"
	"testing"
)

const sampleDiscoveryJSON = `[
  {
    "name": "pdf",
    "status": "installed",
    "source": "anthropics/skills",
    "ref": null,
    "hash": "abc123hash",
    "path": "/tmp/test/.claude/skills/pdf",
    "scope": "project",
    "agents": ["Claude Code"],
    "mode": "copy",
    "security": {
      "gen": "safe",
      "socket": "0 alerts",
      "snyk": "low",
      "details": "https://skills.sh/anthropics/skills"
    }
  },
  {
    "name": "Convex Best Practices",
    "status": "installed",
    "source": "anthropics/skills",
    "ref": null,
    "hash": "def456hash",
    "path": "/tmp/test/.claude/skills/convex-best-practices",
    "scope": "project",
    "agents": ["Claude Code"],
    "mode": "copy",
    "security": {
      "gen": "safe",
      "socket": "0 alerts",
      "snyk": "medium",
      "details": "https://skills.sh/anthropics/skills"
    }
  },
  {
    "name": "failed-skill",
    "status": "failed",
    "source": "anthropics/skills",
    "ref": null,
    "path": "/tmp/test/.claude/skills/failed",
    "security": {
      "gen": "unverified",
      "snyk": "high"
    }
  }
]`

func TestNormaliseSkillID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pdf", "pdf"},
		{"Convex Best Practices", "convex-best-practices"},
		{"  spaced  name  ", "spaced-name"},
		{"My_Awesome-Skill 1.0", "my_awesome-skill-1-0"},
		{"C++ Runner", "c-runner"},
		{"clean-id", "clean-id"},
		{"--leading-and-trailing--", "leading-and-trailing"},
	}

	for _, tt := range tests {
		got := NormaliseSkillID(tt.input)
		if got != tt.want {
			t.Errorf("NormaliseSkillID(%q) = %q, want %q", tt.input, got, tt.want)
		}
		if err := ValidateSkillID(got); err != nil {
			t.Errorf("ValidateSkillID(%q) failed: %v", got, err)
		}
	}
}

func TestParseSkillFrontmatter(t *testing.T) {
	content := []byte(`---
name: pdf
description: Tools for extracting text, tables and structure from PDF documents.
---

# PDF Skill documentation
Here is the rest of the file.
`)

	name, desc, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseSkillFrontmatter error: %v", err)
	}
	if name != "pdf" {
		t.Errorf("name = %q, want %q", name, "pdf")
	}
	wantDesc := "Tools for extracting text, tables and structure from PDF documents."
	if desc != wantDesc {
		t.Errorf("desc = %q, want %q", desc, wantDesc)
	}
}

func TestParseDiscoveredSkills(t *testing.T) {
	skillFiles := map[string][]byte{
		"/tmp/test/.claude/skills/pdf": []byte(`---
name: pdf
description: Extract text and tables from PDF files.
---
# PDF`),
		"/tmp/test/.claude/skills/convex-best-practices": []byte(`---
name: Convex Best Practices
description: Guidelines for Convex backend and query development.
---
# Convex`),
	}

	readMD := func(dirPath string) ([]byte, error) {
		if data, ok := skillFiles[dirPath]; ok {
			return data, nil
		}
		return nil, fmt.Errorf("not found at %s", dirPath)
	}

	source := "anthropics/skills"
	discovered, skipped, err := ParseDiscoveredSkills([]byte(sampleDiscoveryJSON), readMD, source)
	if err != nil {
		t.Fatalf("ParseDiscoveredSkills failed: %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("expected 2 discovered skills, got %d", len(discovered))
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped skill, got %d", len(skipped))
	}

	if skipped[0].Name != "failed-skill" || skipped[0].Status != "failed" {
		t.Errorf("unexpected skipped skill: %+v", skipped[0])
	}

	pdf := discovered[0]
	if string(pdf.ID) != "pdf" {
		t.Errorf("pdf ID = %q, want %q", pdf.ID, "pdf")
	}
	if pdf.Name != "pdf" {
		t.Errorf("pdf Name = %q, want %q", pdf.Name, "pdf")
	}
	if pdf.Snyk != "low" {
		t.Errorf("pdf Snyk = %q, want %q", pdf.Snyk, "low")
	}
	if pdf.Manifest.Ref != source {
		t.Errorf("pdf Ref = %q, want %q", pdf.Manifest.Ref, source)
	}
	if pdf.Manifest.Skill != "pdf" {
		t.Errorf("pdf Skill = %q, want %q", pdf.Manifest.Skill, "pdf")
	}
	if pdf.Manifest.Context == nil || pdf.Manifest.Context.Title != "pdf" || pdf.Manifest.Context.Body != "Extract text and tables from PDF files." {
		t.Errorf("unexpected pdf Context: %+v", pdf.Manifest.Context)
	}

	convex := discovered[1]
	if string(convex.ID) != "convex-best-practices" {
		t.Errorf("convex ID = %q, want %q", convex.ID, "convex-best-practices")
	}
	if convex.Name != "Convex Best Practices" {
		t.Errorf("convex Name = %q, want %q", convex.Name, "Convex Best Practices")
	}
	if convex.Description != "Guidelines for Convex backend and query development." {
		t.Errorf("convex Description = %q, want %q", convex.Description, "Guidelines for Convex backend and query development.")
	}
	if convex.Snyk != "medium" {
		t.Errorf("convex Snyk = %q, want %q", convex.Snyk, "medium")
	}
	if convex.Manifest.Ref != source {
		t.Errorf("convex Ref = %q, want %q", convex.Manifest.Ref, source)
	}
	if convex.Manifest.Skill != "Convex Best Practices" {
		t.Errorf("convex Skill = %q, want %q", convex.Manifest.Skill, "Convex Best Practices")
	}
	if convex.Manifest.Context == nil || convex.Manifest.Context.Body != "Guidelines for Convex backend and query development." {
		t.Errorf("unexpected convex Context: %+v", convex.Manifest.Context)
	}
}

func TestParseDiscoveredSkills_NameDiffersFromDirectory(t *testing.T) {
	rawJSON := `[
	  {
	    "name": "Convex Best Practices",
	    "status": "installed",
	    "source": "owner/repo",
	    "path": "/tmp/custom-path/.claude/skills/convex-best-practices"
	  }
	]`
	dirPath := "/tmp/custom-path/.claude/skills/convex-best-practices"
	skillContent := []byte(`---
name: Convex Best Practices
description: Guidelines and recipes for Convex developers.
---
# Docs`)

	readMD := func(path string) ([]byte, error) {
		if path == dirPath {
			return skillContent, nil
		}
		return nil, fmt.Errorf("wrong path: %s", path)
	}

	discovered, _, err := ParseDiscoveredSkills([]byte(rawJSON), readMD, "owner/repo")
	if err != nil {
		t.Fatalf("ParseDiscoveredSkills: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered skill, got %d", len(discovered))
	}
	if discovered[0].Description != "Guidelines and recipes for Convex developers." {
		t.Errorf("expected description to be resolved via entry.Path directory, got %q", discovered[0].Description)
	}
}
