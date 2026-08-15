package skills

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

//go:embed skills.yml
var rawSkillsYAML []byte

// Spec describes an agent skill installable in a devcontainer workspace.
type Spec struct {
	ID              types.SkillID
	Label           string
	Ref             string
	Skill           string
	RequiresModules []types.ModuleID
	RequiresEnv     []string
	Category        string
	Description     string
	Context         func() *types.ContextSection
}

// Category represents a functional grouping of related skills.
type Category struct {
	ID      types.SkillID
	Label   string
	Aliases []string
	Skills  []types.SkillID
	Specs   []*Spec
}

var (
	// All is the complete ordered catalogue of installable agent skills.
	All []*Spec

	// Categories contains the 6 semantic functional categories.
	Categories []*Category
)

type rawManifest struct {
	Categories []rawCategory `yaml:"categories"`
	Skills     []rawSkill    `yaml:"skills"`
}

type rawCategory struct {
	ID      string   `yaml:"id"`
	Label   string   `yaml:"label"`
	Aliases []string `yaml:"aliases"`
	Skills  []string `yaml:"skills"`
}

type rawSkill struct {
	ID              string   `yaml:"id"`
	Label           string   `yaml:"label"`
	Ref             string   `yaml:"ref"`
	Skill           string   `yaml:"skill"`
	RequiresModules []string `yaml:"requires_modules"`
	RequiresEnv     []string `yaml:"requires_env"`
	Category        string   `yaml:"category"`
	Description     string   `yaml:"description"`
}

func init() {
	var manifest rawManifest
	if err := yaml.Unmarshal(rawSkillsYAML, &manifest); err != nil {
		panic(fmt.Sprintf("skills: invalid embedded skills.yml: %v", err))
	}

	byID := make(map[types.SkillID]*Spec, len(manifest.Skills))

	for _, raw := range manifest.Skills {
		sID := types.SkillID(raw.ID)
		reqModules := make([]types.ModuleID, 0, len(raw.RequiresModules))
		for _, m := range raw.RequiresModules {
			reqModules = append(reqModules, types.ModuleID(m))
		}
		if len(reqModules) == 0 {
			reqModules = []types.ModuleID{types.ModuleNodejs}
		}

		spec := &Spec{
			ID:              sID,
			Label:           raw.Label,
			Ref:             raw.Ref,
			Skill:           raw.Skill,
			RequiresModules: reqModules,
			RequiresEnv:     raw.RequiresEnv,
			Category:        raw.Category,
			Description:     strings.TrimSpace(raw.Description),
		}

		title := string(spec.ID)
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:] + " skill"
		}
		body := spec.Description
		if body == "" {
			body = fmt.Sprintf("Agent skill (%s) installed in this workspace.", spec.ID)
		}
		spec.Context = func() *types.ContextSection {
			return &types.ContextSection{
				Title: title,
				Body:  body,
			}
		}

		All = append(All, spec)
		byID[sID] = spec
	}

	for _, rawCat := range manifest.Categories {
		catID := types.SkillID(rawCat.ID)
		cat := &Category{
			ID:      catID,
			Label:   rawCat.Label,
			Aliases: rawCat.Aliases,
		}
		for _, s := range rawCat.Skills {
			skID := types.SkillID(s)
			cat.Skills = append(cat.Skills, skID)
			if spec, ok := byID[skID]; ok {
				cat.Specs = append(cat.Specs, spec)
			}
		}
		Categories = append(Categories, cat)
	}
}

// InstallRef is the entry the installer receives: the source, with the skill
// selector appended when the source holds more than one. It stays a single
// token because the entries travel space-separated in the environment.
func (s *Spec) InstallRef() string {
	if s.Skill == "" {
		return s.Ref
	}
	return s.Ref + types.SkillRefSeparator + s.Skill
}

// Get returns the spec for an id, or nil when the id is unknown.
func Get(id types.SkillID) *Spec {
	for _, s := range All {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// GetCategory returns the category matching id or one of its aliases, or nil.
func GetCategory(id types.SkillID) *Category {
	target := string(id)
	for _, c := range Categories {
		if c.ID == id {
			return c
		}
		for _, alias := range c.Aliases {
			if alias == target {
				return c
			}
		}
	}
	return nil
}
