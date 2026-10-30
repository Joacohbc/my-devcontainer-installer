package types

// UserSkillManifest is the on-disk YAML shape of a user-defined agent skill:
// saved as ~/.devcontainer-cli/skills/<id>.yml.
type UserSkillManifest struct {
	ID              string            `yaml:"id"`
	Label           string            `yaml:"label,omitempty"`
	Ref             string            `yaml:"ref"`
	Skill           string            `yaml:"skill,omitempty"`
	RequiresModules []string          `yaml:"requires_modules,omitempty"`
	Context         *UserSkillContext `yaml:"context,omitempty"`
}

// UserSkillContext becomes the skill's entry in the generated ~/CONTEXT.md.
type UserSkillContext struct {
	Title string `yaml:"title"`
	Body  string `yaml:"body"`
}
