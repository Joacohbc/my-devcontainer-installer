package service

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// modeChoiceCustomFromProfile is the wizard-only third mode-step option: still
// types.BuildModeCustom underneath (currentMode normalizes it), but it also
// triggers the profile-picker step that plain "custom" skips. It has no
// meaning past the wizard — reduce() never stores it, only the normalized
// BuildMode.
const modeChoiceCustomFromProfile = "custom-from-profile"

var modeChoiceLabels = map[string]string{
	string(types.BuildModeCustom):   "custom — pick modules yourself, build locally",
	modeChoiceCustomFromProfile:     "custom, from a profile — start from a saved module bundle, build locally",
	string(types.BuildModeProfiles): "profiles — skip the build, pull a pre-built image for a catalog profile",
}

const (
	stepKeyWorkspace     = "workspace"
	stepKeyMode          = "mode"
	stepKeyProfile       = "profile"
	stepKeyVariant       = "variant"
	stepKeyImage         = "image"
	stepKeySubnet        = "subnet"
	stepKeyPorts         = "ports"
	stepKeyVolumes       = "volumes"
	stepKeySharedConfig  = "sharedConfig"
	stepKeySkillsEnabled = "skillsEnabled"
	stepKeySkillsMode    = "skillsMode"
)

const (
	// skillsPerPage caps one screen's worth of skills, so a category with dozens
	// of them is asked page by page instead of as one unreadable multiselect.
	skillsPerPage = 10
	// skipSkillCategory is the first choice of every skill page: it drops the
	// whole category, not just the page. ':' is outside the charset
	// domain.ValidateSkillID accepts, so it can never collide with a skill id.
	skipSkillCategory = "skip:category"
)

func categoryStepKey(c types.UICategory) string  { return "cat:" + string(c) }
func optionStepKey(entryID, optID string) string { return "opt:" + entryID + ":" + optID }
func envStepKey(name string) string              { return "env:" + name }

func skillStepKey(category types.SkillID, page int) string {
	return fmt.Sprintf("skills:%s:%d", category, page)
}

func choicesFromOption(o types.ModuleOption) []Option {
	choices := make([]Option, len(o.Choices))
	for i, c := range o.Choices {
		choices[i] = Option{Value: c.Value, Label: c.Label}
	}
	return choices
}

func defaultStrings(v any) []string {
	return types.CoerceStrings(v)
}

// ServiceOptionsOf returns the stored options for the compose service id within
// a config's service list.
func ServiceOptionsOf(services []types.SelectedService, id string) map[string]any {
	for _, s := range services {
		if string(s.ID) == id {
			return s.Options
		}
	}
	return map[string]any{}
}

func optionDefault(o types.ModuleOption, prevOpts map[string]any) any {
	if prevOpts != nil {
		if v, ok := prevOpts[o.ID]; ok && v != nil {
			return v
		}
	}
	return o.Default
}

func moduleOptionsOf(modules []types.SelectedModule, id string) map[string]any {
	for _, m := range modules {
		if string(m.ID) == id {
			return m.Options
		}
	}
	return map[string]any{}
}

func optionField(o types.ModuleOption, initial any) Field {
	switch o.Type {
	case types.ModuleOptionMultiselect:
		return Field{Kind: FieldMultiselect, Title: "  " + o.Label + ":", Choices: choicesFromOption(o), Initial: defaultStrings(initial)}
	case types.ModuleOptionConfirm:
		// No is the wizard's answer to a yes/no it was told nothing about; an
		// option that wants otherwise says so in its Default.
		enabled, _ := initial.(bool)
		return Field{Kind: FieldConfirm, Title: "  " + o.Label + "?", Initial: enabled}
	case types.ModuleOptionSelect:
		s, _ := initial.(string)
		return Field{Kind: FieldSelect, Title: "  " + o.Label + ":", Choices: choicesFromOption(o), Initial: s}
	default:
		s, _ := initial.(string)
		return Field{Kind: FieldInput, Title: "  " + o.Label + ":", Initial: s}
	}
}

func optionAnswer(s *State, key string, o types.ModuleOption) any {
	switch o.Type {
	case types.ModuleOptionMultiselect:
		return s.Strings(key)
	case types.ModuleOptionConfirm:
		return s.Bool(key)
	default:
		return s.String(key)
	}
}

// currentMode normalizes the raw wizard mode-choice answer (3 options: plain
// custom, custom-from-a-profile, or profiles) down to the 2 real
// types.BuildMode values used everywhere past this step. Only the literal
// "profiles" choice means profiles mode; every other raw value (including the
// unanswered "") is some flavor of custom.
func currentMode(s *State) types.BuildMode {
	if s.String(stepKeyMode) == string(types.BuildModeProfiles) {
		return types.BuildModeProfiles
	}
	return types.BuildModeCustom
}

func categoryChoices(c types.UICategory, mode types.BuildMode, selectedModules map[string]bool) []Option {
	var choices []Option
	for _, e := range catalog.SelectableByCategory()[c] {
		if !e.IsService && mode != types.BuildModeCustom {
			continue
		}
		if e.IsService {
			if svc := catalog.GetComposeService(types.ServiceID(e.ID)); svc != nil && svc.RequiresModule != "" && !selectedModules[string(svc.RequiresModule)] {
				continue
			}
		}
		choices = append(choices, Option{Value: e.ID, Label: e.Label})
	}
	return choices
}

func selectedEntryIDs(s *State) map[string]bool {
	set := map[string]bool{}
	for _, c := range catalog.CategoriesInOrder() {
		for _, id := range s.Strings(categoryStepKey(c)) {
			set[id] = true
		}
	}
	return set
}

func selectedModuleIDs(s *State) map[string]bool {
	set := map[string]bool{}
	for id := range selectedEntryIDs(s) {
		if catalog.GetDockerfileModule(types.ModuleID(id)) != nil {
			set[id] = true
		}
	}
	return set
}

func baseCategoryIDs(base *types.DevcontainerConfig, c types.UICategory) []string {
	var ids []string
	for _, m := range base.Dockerfile.Modules {
		if spec := catalog.GetDockerfileModule(m.ID); spec != nil && spec.UICategory == c {
			ids = append(ids, string(m.ID))
		}
	}
	for _, s := range base.Compose.Services {
		if spec := catalog.GetComposeService(s.ID); spec != nil && spec.UICategory == c {
			ids = append(ids, string(s.ID))
		}
	}
	return ids
}

type wizardContext struct {
	base            *types.DevcontainerConfig
	workspace       string
	usedSubnets     []domain.CidrRange
	suggestedSubnet string
	// allProfiles is every known profile — built-in and the user's own — for
	// the "custom, from a profile" picker (profileStep). Unlike the module
	// category steps, which only ever offer catalog module ids, this one lists
	// whole profiles, so it needs the full catalog rather than just modules.
	allProfiles []catalog.Profile
}

func (w wizardContext) steps(s *State) []Step {
	steps := []Step{w.workspaceStep(), w.modeStep()}

	switch s.String(stepKeyMode) {
	case string(types.BuildModeProfiles):
		steps = append(steps, w.variantStep())
	case modeChoiceCustomFromProfile:
		// Only appears when the user explicitly chose to start from a profile
		// (built-in or their own); it pre-selects that profile's modules before
		// the per-category steps.
		if step, ok := w.profileStep(); ok {
			steps = append(steps, step)
		}
	}

	steps = append(steps, w.categorySteps(s)...)
	steps = append(steps, w.optionSteps(s)...)

	if !w.remoteImageDerived(s) {
		steps = append(steps, w.imageStep())
	}
	steps = append(steps, w.subnetStep())
	steps = append(steps, w.portsStep())
	steps = append(steps, w.volumesStep())
	steps = append(steps, w.sharedConfigStep())
	steps = append(steps, w.skillSteps(s)...)
	steps = append(steps, w.envSteps(s)...)
	return steps
}

// parsePortsCSV splits a comma-separated list of docker port specs, trimming
// blanks. Specs are stored verbatim; the generator binds those without an
// explicit host IP to 127.0.0.1.
func parsePortsCSV(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (w wizardContext) portsStep() Step {
	return Step{Key: stepKeyPorts, Build: func(s *State) Field {
		initial := strings.Join(w.base.Compose.Ports, ",")
		if s.Has(stepKeyPorts) {
			initial = s.String(stepKeyPorts)
		}
		return Field{
			Kind:    FieldInput,
			Title:   "Ports to publish on the devcontainer (e.g. 8080:80,5432:5432 — bound to 127.0.0.1; empty for none):",
			Initial: initial,
		}
	}}
}

// parseVolumesCSV splits a comma-separated list of docker volume specs,
// trimming blanks. Specs are stored verbatim and threaded into the devcontainer
// service by the generator.
func parseVolumesCSV(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (w wizardContext) volumesStep() Step {
	return Step{Key: stepKeyVolumes, Build: func(s *State) Field {
		initial := strings.Join(w.base.Compose.Volumes, ",")
		if s.Has(stepKeyVolumes) {
			initial = s.String(stepKeyVolumes)
		}
		return Field{
			Kind:    FieldInput,
			Title:   "Extra volumes to mount on the devcontainer (e.g. myvol:/data,./cache:/cache — empty for none):",
			Initial: initial,
		}
	}}
}

func (w wizardContext) sharedConfigStep() Step {
	return Step{Key: stepKeySharedConfig, Build: func(s *State) Field {
		// Every wizard yes/no starts at no, so an unset project (a fresh one)
		// defaults to not mounting it; one that already chose keeps its choice.
		initial := w.base.Compose.SharedConfig != nil && *w.base.Compose.SharedConfig
		if s.Has(stepKeySharedConfig) {
			initial = s.Bool(stepKeySharedConfig)
		}
		return Field{
			Kind:    FieldConfirm,
			Title:   "Mount the global shared config volume (logins/sessions for Claude, gh, codex… persist across containers)?",
			Initial: initial,
		}
	}}
}

func (w wizardContext) workspaceStep() Step {
	return Step{Key: stepKeyWorkspace, Build: func(s *State) Field {
		initial := w.workspace
		if s.Has(stepKeyWorkspace) {
			initial = s.String(stepKeyWorkspace)
		}
		return Field{
			Kind:    FieldInput,
			Title:   "Workspace name (used as prefix for containers, network, volumes):",
			Initial: initial,
			Validate: func(v string) error {
				if !domain.IsValidDockerName(v) {
					return fmt.Errorf("invalid Docker name (allowed: a-z A-Z 0-9 _ . -, ≤63 chars, start alphanumeric)")
				}
				return nil
			},
		}
	}}
}

// modeStep offers all 3 starting points up front (plain custom, custom from a
// profile, profiles/remote) rather than nesting the profile choice as an
// auto-appearing sub-step of custom — see modeChoiceCustomFromProfile.
func (w wizardContext) modeStep() Step {
	return Step{Key: stepKeyMode, Build: func(s *State) Field {
		modeChoiceOrder := []string{string(types.BuildModeCustom), modeChoiceCustomFromProfile, string(types.BuildModeProfiles)}
		choices := make([]Option, len(modeChoiceOrder))
		for i, m := range modeChoiceOrder {
			choices[i] = Option{Value: m, Label: modeChoiceLabels[m]}
		}
		// An existing project only ever persisted a real BuildMode, never the
		// wizard-only "from a profile" choice, so re-entering the wizard always
		// defaults to plain custom/profiles rather than guessing at intent.
		initial := string(w.base.Mode)
		if initial == "" {
			initial = string(types.BuildModeCustom)
		}
		if s.Has(stepKeyMode) {
			initial = s.String(stepKeyMode)
		}
		return Field{Kind: FieldSelect, Title: "Build mode:", Choices: choices, Initial: initial}
	}}
}

func (w wizardContext) variantStep() Step {
	return Step{Key: stepKeyVariant, Build: func(s *State) Field {
		initial := "nodejs"
		if w.base.Remote != nil && w.base.Remote.Variant != "" {
			initial = w.base.Remote.Variant
		}
		if s.Has(stepKeyVariant) {
			initial = s.String(stepKeyVariant)
		}
		return Field{Kind: FieldSelect, Title: "Profile:", Choices: ProfileChoices(domain.ProfileDirs()...), Initial: initial}
	}}
}

// profileStep lets the user start from any known profile — built-in or their
// own. It only appears when they explicitly picked the "custom, from a
// profile" mode choice (see steps()), never as an auto-appended extra
// question. Selecting one seeds the per-category module multiselects.
func (w wizardContext) profileStep() (Step, bool) {
	if len(w.allProfiles) == 0 {
		return Step{}, false
	}
	return Step{Key: stepKeyProfile, Build: func(s *State) Field {
		choices := []Option{{Value: "", Label: "(none — choose modules manually)"}}
		for _, p := range w.allProfiles {
			label := p.ID
			if len(p.Modules) > 0 {
				label = fmt.Sprintf("%s (%s)", p.ID, strings.Join(p.Modules, ", "))
			}
			choices = append(choices, Option{Value: p.ID, Label: label})
		}
		initial := ""
		if s.Has(stepKeyProfile) {
			initial = s.String(stepKeyProfile)
		}
		return Field{Kind: FieldSelect, Title: "Start from which profile?:", Choices: choices, Initial: initial}
	}}, true
}

func (w wizardContext) profileByID(id string) (catalog.Profile, bool) {
	for _, p := range w.allProfiles {
		if p.ID == id {
			return p, true
		}
	}
	return catalog.Profile{}, false
}

// pickedProfile is the profile chosen in profileStep, if any. Every step that
// comes after it seeds from it through this one accessor — modules, scripts,
// skills and skills mode alike — so "start from this profile" means the same
// thing for all of them instead of each step deciding for itself.
func (w wizardContext) pickedProfile(s *State) (catalog.Profile, bool) {
	pid := s.String(stepKeyProfile)
	if pid == "" {
		return catalog.Profile{}, false
	}
	return w.profileByID(pid)
}

// profileCategoryIDs returns the profile's module ids that belong to UI category c.
func profileCategoryIDs(p catalog.Profile, c types.UICategory) []string {
	var ids []string
	for _, id := range p.Modules {
		if spec := catalog.GetDockerfileModule(types.ModuleID(id)); spec != nil && spec.UICategory == c {
			ids = append(ids, id)
		}
	}
	return ids
}

func (w wizardContext) categorySteps(s *State) []Step {
	var steps []Step
	for _, c := range catalog.CategoriesInOrder() {
		if len(categoryChoices(c, currentMode(s), selectedModuleIDs(s))) == 0 {
			continue
		}
		category := c
		steps = append(steps, Step{Key: categoryStepKey(category), Build: func(s *State) Field {
			// Seed from base, or from the chosen profile's modules in this
			// category. An explicit answer for this category always wins.
			initial := baseCategoryIDs(w.base, category)
			if p, ok := w.pickedProfile(s); ok {
				initial = profileCategoryIDs(p, category)
			}
			if s.Has(categoryStepKey(category)) {
				initial = s.Strings(categoryStepKey(category))
			}
			return Field{
				Kind:    FieldMultiselect,
				Title:   types.UICategoryLabels[category] + " (Space to select, Enter to confirm):",
				Choices: categoryChoices(category, currentMode(s), selectedModuleIDs(s)),
				Initial: initial,
			}
		}})
	}
	return steps
}

func (w wizardContext) optionSteps(s *State) []Step {
	selected := selectedEntryIDs(s)
	localCached := currentMode(s) == types.BuildModeCustom
	var steps []Step
	for _, m := range catalog.DockerfileModules {
		// Always-on modules (e.g. base) never appear in the category multiselects,
		// so they are never "selected" there — but their own options (e.g. the
		// default Powerlevel10k style) must still be offered.
		if !localCached || (!selected[string(m.ID)] && !m.Always) {
			continue
		}
		steps = append(steps, w.entryOptionSteps(string(m.ID), m.Options, moduleOptionsOf(w.base.Dockerfile.Modules, string(m.ID)))...)
	}
	for _, svc := range catalog.ComposeServices {
		if !selected[string(svc.ID)] {
			continue
		}
		steps = append(steps, w.entryOptionSteps(string(svc.ID), svc.Options, ServiceOptionsOf(w.base.Compose.Services, string(svc.ID)))...)
	}
	return steps
}

func (w wizardContext) entryOptionSteps(entryID string, options []types.ModuleOption, prevOpts map[string]any) []Step {
	var steps []Step
	for _, o := range options {
		option := o
		key := optionStepKey(entryID, option.ID)
		steps = append(steps, Step{Key: key, Build: func(s *State) Field {
			var initial any = optionDefault(option, prevOpts)
			if s.Has(key) {
				initial = optionAnswer(s, key, option)
			}
			return optionField(option, initial)
		}})
	}
	return steps
}

func (w wizardContext) remoteImageDerived(s *State) bool {
	return currentMode(s) == types.BuildModeProfiles && s.String(stepKeyVariant) != ""
}

func (w wizardContext) imageStep() Step {
	return Step{Key: stepKeyImage, Build: func(s *State) Field {
		initial := w.base.Image
		if s.Has(stepKeyImage) {
			initial = s.String(stepKeyImage)
		}
		return Field{
			Kind:    FieldInput,
			Title:   "Image name:",
			Initial: initial,
			Validate: func(v string) error {
				if !domain.IsValidImageName(v) {
					return fmt.Errorf("invalid Docker image name")
				}
				return nil
			},
		}
	}}
}

func (w wizardContext) subnetStep() Step {
	return Step{Key: stepKeySubnet, Build: func(s *State) Field {
		initial := w.suggestedSubnet
		if s.Has(stepKeySubnet) {
			initial = s.String(stepKeySubnet)
		}
		return Field{
			Kind:    FieldInput,
			Title:   "Docker network subnet (CIDR):",
			Initial: initial,
			Validate: func(v string) error {
				if !domain.IsValidCidr(v) {
					return fmt.Errorf("invalid CIDR")
				}
				if clash := domain.SubnetConflict(v, w.usedSubnets); clash != nil {
					return fmt.Errorf("overlaps with existing network %s", domain.FormatCidr(*clash))
				}
				return nil
			},
		}
	}}
}

func (w wizardContext) envSteps(s *State) []Step {
	var steps []Step
	for _, e := range domain.CollectRequiredEnvVars(w.reduce(s)) {
		envVar := e
		steps = append(steps, Step{Key: envStepKey(envVar.Name), Build: func(s *State) Field {
			initial := w.base.Env[envVar.Name]
			if initial == "" {
				initial = envVar.Default
			}
			if s.Has(envStepKey(envVar.Name)) {
				initial = s.String(envStepKey(envVar.Name))
			}
			return Field{Kind: FieldInput, Title: envVar.Prompt + ":", Initial: initial}
		}})
	}
	return steps
}

// reduce folds the gathered wizard answers into a DevcontainerConfig. It is
// called both mid-wizard (to discover required env vars from the current
// selection) and once at the end to produce the final config.
func (w wizardContext) reduce(s *State) *types.DevcontainerConfig {
	mode := currentMode(s)
	selected := selectedEntryIDs(s)

	var modules []types.SelectedModule
	if mode == types.BuildModeCustom {
		for _, m := range catalog.DockerfileModules {
			// Always-on modules are never part of the category selection (see
			// optionSteps), but their own answered options must still be saved.
			if !selected[string(m.ID)] && !m.Always {
				continue
			}
			modules = append(modules, types.SelectedModule{ID: m.ID, Options: w.entryOptions(s, string(m.ID), m.Options, moduleOptionsOf(w.base.Dockerfile.Modules, string(m.ID)))})
		}
	}

	var services []types.SelectedService
	for _, svc := range catalog.ComposeServices {
		if !selected[string(svc.ID)] {
			continue
		}
		services = append(services, types.SelectedService{ID: svc.ID, Options: w.entryOptions(s, string(svc.ID), svc.Options, ServiceOptionsOf(w.base.Compose.Services, string(svc.ID)))})
	}

	workspace := w.workspace
	if s.Has(stepKeyWorkspace) {
		workspace = s.String(stepKeyWorkspace)
	}

	env := map[string]string{}
	for k, v := range w.base.Env {
		env[k] = v
	}

	draft := &types.DevcontainerConfig{
		Mode:       mode,
		Image:      s.String(stepKeyImage),
		Workspace:  workspace,
		Dockerfile: types.DockerfileConfig{Modules: modules, Scripts: w.selectedScripts(s)},
		Skills:     w.selectedSkills(s),
		Compose:    types.ComposeConfig{Services: services, Subnet: s.String(stepKeySubnet)},
		Env:        env,
	}

	if s.Has(stepKeyPorts) {
		draft.Compose.Ports = parsePortsCSV(s.String(stepKeyPorts))
	} else {
		draft.Compose.Ports = w.base.Compose.Ports
	}

	if s.Has(stepKeyVolumes) {
		draft.Compose.Volumes = parseVolumesCSV(s.String(stepKeyVolumes))
	} else {
		draft.Compose.Volumes = w.base.Compose.Volumes
	}

	if s.Has(stepKeySharedConfig) {
		v := s.Bool(stepKeySharedConfig)
		draft.Compose.SharedConfig = &v
	} else {
		draft.Compose.SharedConfig = w.base.Compose.SharedConfig
	}

	if mode == types.BuildModeProfiles {
		variant := s.String(stepKeyVariant)
		registry := ""
		if w.base.Remote != nil {
			registry = w.base.Remote.Registry
		}
		if variant != "" {
			draft.Remote = &types.RemoteConfig{Variant: variant, Registry: registry}
			draft.Image = domain.ResolveRemoteImage(variant, domain.ResolveRegistry("", registry))
		}
	}

	for _, e := range domain.CollectRequiredEnvVars(draft) {
		if v := s.String(envStepKey(e.Name)); v != "" {
			env[e.Name] = v
		}
	}
	return draft
}

// selectedScripts are the custom scripts of the profile picked in the wizard,
// or the ones the project already carried when no profile was picked. Picking a
// profile replaces them rather than merging: the wizard's profile step is
// "start from this profile", so leaving the previous profile's scripts behind
// would quietly build an image neither profile describes.
//
// Profiles mode builds no image here, so it carries no scripts either.
func (w wizardContext) selectedScripts(s *State) []types.CustomScript {
	if currentMode(s) != types.BuildModeCustom {
		return nil
	}
	p, ok := w.pickedProfile(s)
	if !ok {
		return w.base.Dockerfile.Scripts
	}
	scripts, err := domain.ProfileScripts(p)
	if err != nil {
		// A broken profile must not silently drop the project's own scripts; the
		// generate flow validates and reports them right after.
		return w.base.Dockerfile.Scripts
	}
	return scripts
}

func (w wizardContext) entryOptions(s *State, entryID string, options []types.ModuleOption, prevOpts map[string]any) map[string]any {
	opts := map[string]any{}
	for _, o := range options {
		key := optionStepKey(entryID, o.ID)
		if s.Has(key) {
			opts[o.ID] = optionAnswer(s, key, o)
		} else {
			opts[o.ID] = optionDefault(o, prevOpts)
		}
	}
	return opts
}

// Configure runs the interactive wizard (via the Prompter) starting from base
// and returns the resolved config. cwd seeds the default workspace name and the
// subnet suggestion from the project location.
func (s GenerateService) Configure(base *types.DevcontainerConfig, cwd string, prompt Prompter) (*types.DevcontainerConfig, error) {
	workspace := base.Workspace
	if workspace == "" {
		workspace = domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer")
	}

	usedSubnets := domain.ListUsedSubnets(captureFunc(), domain.WorkspaceNetworkName(workspace))
	preferredSubnet := base.Compose.Subnet
	if preferredSubnet == "" {
		preferredSubnet = domain.DefaultSubnet
	}

	ctx := wizardContext{
		base:            base,
		workspace:       workspace,
		usedSubnets:     usedSubnets,
		suggestedSubnet: domain.FindFreeSubnet(preferredSubnet, usedSubnets),
		allProfiles:     catalog.All(domain.ProfileDirs()...),
	}

	state, err := prompt.Wizard(ctx.steps)
	if err != nil {
		return nil, err
	}
	return ctx.reduce(state), nil
}

// moduleChoices lists the selectable Dockerfile modules in a UI category,
// excluding compose services (a profile is a pure module bundle).
func moduleChoices(c types.UICategory) []Option {
	var out []Option
	for _, e := range catalog.SelectableByCategory()[c] {
		if e.IsService {
			continue
		}
		out = append(out, Option{Value: e.ID, Label: e.Label})
	}
	return out
}

// baseModuleIDs returns the ids of base's Dockerfile modules belonging to a UI
// category, used to pre-select them in the module-only wizard.
func baseModuleIDs(base *types.DevcontainerConfig, c types.UICategory) []string {
	var ids []string
	for _, m := range base.Dockerfile.Modules {
		if spec := catalog.GetDockerfileModule(m.ID); spec != nil && spec.UICategory == c {
			ids = append(ids, string(m.ID))
		}
	}
	return ids
}

// SelectModules runs a trimmed wizard that only offers the per-category module
// multiselects (no services, options, workspace, ports, volumes, …) and returns
// the chosen module ids in catalog order. It backs `config profile create`, where
// a profile is just a reusable bundle of modules.
func (s GenerateService) SelectModules(base *types.DevcontainerConfig, prompt Prompter) ([]string, error) {
	build := func(*State) []Step {
		var steps []Step
		for _, c := range catalog.CategoriesInOrder() {
			choices := moduleChoices(c)
			if len(choices) == 0 {
				continue
			}
			category := c
			steps = append(steps, Step{Key: categoryStepKey(category), Build: func(st *State) Field {
				initial := baseModuleIDs(base, category)
				if st.Has(categoryStepKey(category)) {
					initial = st.Strings(categoryStepKey(category))
				}
				return Field{
					Kind:    FieldMultiselect,
					Title:   types.UICategoryLabels[category] + " (Space to select, Enter to confirm):",
					Choices: choices,
					Initial: initial,
				}
			}})
		}
		return steps
	}

	state, err := prompt.Wizard(build)
	if err != nil {
		return nil, err
	}

	selected := selectedModuleIDs(state)
	var ids []string
	for _, m := range catalog.DockerfileModules {
		if selected[string(m.ID)] {
			ids = append(ids, string(m.ID))
		}
	}
	return ids, nil
}

// ProfileChoices lists the profiles usable for --profile under the "profiles"
// build mode: those with Profile.Remote set, i.e. a published
// ghcr.io/devcontainer-<id> image built by CI. A profile without one (e.g.
// 'scraper') can still be applied locally via --profile in mode=custom, but
// is left out here — offering it would just fail the pull.
func ProfileChoices(dirs ...string) []Option {
	var choices []Option
	for _, p := range catalog.All(dirs...) {
		if !p.Remote {
			continue
		}
		label := p.ID
		if p.Label != "" {
			label = fmt.Sprintf("%s — %s", p.ID, p.Label)
		}
		choices = append(choices, Option{Value: p.ID, Label: label})
	}
	return choices
}

// skillSteps let the project pick agent skills and how they get installed. A
// yes/no gate comes first, and past it they are asked one category at a time in
// pages of skillsPerPage: the catalogue runs to dozens of entries, and one
// multiselect that long is unreadable. The
// mode step only appears once a skill is selected, since it has nothing to
// govern otherwise. Profiles mode is skipped: the installer and its alias come
// from a Dockerfile module, and a prebuilt image was not built with it.
func (w wizardContext) skillSteps(s *State) []Step {
	groups := catalog.AgentSkillsByCategory(domain.SkillDirs()...)
	if currentMode(s) != types.BuildModeCustom || len(groups) == 0 {
		return nil
	}
	// One yes/no before any listing: a project that wants no skills answers it
	// once instead of paging through every category to select nothing.
	steps := []Step{w.skillsEnabledStep()}
	if !s.Bool(stepKeySkillsEnabled) {
		return steps
	}
	for _, group := range groups {
		steps = append(steps, w.skillCategorySteps(s, group)...)
	}
	if len(pickedSkillIDs(s)) > 0 {
		steps = append(steps, w.skillsModeStep())
	}
	return steps
}

func (w wizardContext) skillsEnabledStep() Step {
	return Step{Key: stepKeySkillsEnabled, Build: func(s *State) Field {
		initial := len(w.initialSkillIDs(s)) > 0
		if s.Has(stepKeySkillsEnabled) {
			initial = s.Bool(stepKeySkillsEnabled)
		}
		return Field{
			Kind:    FieldConfirm,
			Title:   "Install agent skills into this project?",
			Initial: initial,
		}
	}}
}

// skillCategorySteps is one step per page of a category. Answering the skip
// sentinel drops the pages after it: the sentinel means the whole category, so
// there is nothing left to ask about it.
func (w wizardContext) skillCategorySteps(s *State, group catalog.SkillGroup) []Step {
	var steps []Step
	for page, key := range skillStepKeys(group) {
		steps = append(steps, Step{Key: key, Build: w.skillPageField(group, page)})
		if slices.Contains(s.Strings(key), skipSkillCategory) {
			break
		}
	}
	return steps
}

func (w wizardContext) skillPageField(group catalog.SkillGroup, page int) func(*State) Field {
	from := page * skillsPerPage
	to := min(from+skillsPerPage, len(group.Skills))
	return func(s *State) Field {
		choices := []Option{{Value: skipSkillCategory, Label: "Skip this category — install none of its skills"}}
		pageIDs := make([]string, 0, to-from)
		for _, spec := range group.Skills[from:to] {
			choices = append(choices, Option{Value: string(spec.ID), Label: spec.Label})
			pageIDs = append(pageIDs, string(spec.ID))
		}
		initial := intersectStrings(pageIDs, w.initialSkillIDs(s))
		if key := skillStepKey(group.ID, page); s.Has(key) {
			initial = s.Strings(key)
		}
		return Field{
			Kind:    FieldMultiselect,
			Title:   skillPageTitle(group, from, to),
			Choices: choices,
			Initial: initial,
		}
	}
}

func skillPageTitle(group catalog.SkillGroup, from, to int) string {
	if len(group.Skills) <= skillsPerPage {
		return fmt.Sprintf("%s (Space to select, Enter to confirm):", group.Label)
	}
	return fmt.Sprintf("%s — %d-%d of %d (Space to select, Enter to confirm):", group.Label, from+1, to, len(group.Skills))
}

func skillStepKeys(group catalog.SkillGroup) []string {
	pages := (len(group.Skills) + skillsPerPage - 1) / skillsPerPage
	keys := make([]string, 0, pages)
	for page := 0; page < pages; page++ {
		keys = append(keys, skillStepKey(group.ID, page))
	}
	return keys
}

// initialSkillIDs is what the pages arrive pre-checked with: the chosen
// profile's own skills, or the project's existing ones. A profile that declares
// skills means them the same way it means its modules, so they are not
// something to re-pick by hand.
func (w wizardContext) initialSkillIDs(s *State) []string {
	if p, ok := w.pickedProfile(s); ok {
		return skillIDStrings(p.Skills)
	}
	return skillIDStrings(w.base.Skills.Skills)
}

func intersectStrings(ids, want []string) []string {
	var out []string
	for _, id := range ids {
		if slices.Contains(want, id) {
			out = append(out, id)
		}
	}
	return out
}

// pickedSkillIDs is every skill the category pages selected. A category whose
// pages carry the skip sentinel contributes nothing, including what earlier
// pages of it had selected before the user went back and skipped it.
func pickedSkillIDs(s *State) []string {
	var out []string
	for _, group := range catalog.AgentSkillsByCategory(domain.SkillDirs()...) {
		out = append(out, categorySkillIDs(s, group)...)
	}
	return out
}

func categorySkillIDs(s *State, group catalog.SkillGroup) []string {
	var out []string
	for _, key := range skillStepKeys(group) {
		answered := s.Strings(key)
		if slices.Contains(answered, skipSkillCategory) {
			return nil
		}
		out = append(out, answered...)
	}
	return out
}

func (w wizardContext) skillsModeStep() Step {
	return Step{Key: stepKeySkillsMode, Build: func(s *State) Field {
		choices := []Option{
			{Value: string(types.SkillModeAuto), Label: "auto — install them on every container start"},
			{Value: string(types.SkillModeManual), Label: "manual — leave the install_skills command for you"},
		}
		initial := string(w.base.Skills.ResolvedMode())
		// A profile that pinned a mode carries it too — its skills and the way
		// they install are one decision, not two.
		if p, ok := w.pickedProfile(s); ok && p.SkillsMode != "" {
			initial = string(p.SkillsMode)
		}
		if s.Has(stepKeySkillsMode) {
			initial = s.String(stepKeySkillsMode)
		}
		return Field{Kind: FieldSelect, Title: "When should the skills install?", Choices: choices, Initial: initial}
	}}
}

func (w wizardContext) selectedSkills(s *State) types.SkillsConfig {
	if currentMode(s) != types.BuildModeCustom {
		return types.SkillsConfig{}
	}
	// The skills steps normally always run in custom mode; when they did not,
	// fall back the same way selectedScripts does — to the chosen profile's
	// own skills, or the project's existing ones.
	if !s.Has(stepKeySkillsEnabled) {
		if p, ok := w.pickedProfile(s); ok {
			return types.SkillsConfig{Mode: p.SkillsMode, Skills: p.Skills}
		}
		return w.base.Skills
	}
	// Answering the gate with "no" is an answer, not a missing one: it clears
	// what the project or the profile brought rather than falling back to it.
	if !s.Bool(stepKeySkillsEnabled) {
		return types.SkillsConfig{}
	}
	picked := pickedSkillIDs(s)
	if len(picked) == 0 {
		return types.SkillsConfig{}
	}
	selected := make([]types.SkillID, 0, len(picked))
	for _, id := range picked {
		selected = append(selected, types.SkillID(id))
	}
	mode := w.base.Skills.Mode
	if p, ok := w.pickedProfile(s); ok && p.SkillsMode != "" {
		mode = p.SkillsMode
	}
	if s.Has(stepKeySkillsMode) {
		mode = types.SkillMode(s.String(stepKeySkillsMode))
	}
	return types.SkillsConfig{Mode: mode, Skills: selected}
}

func skillIDStrings(ids []types.SkillID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

// SelectSkills runs a skills-only wizard: which agent skills a profile carries
// and how a project using it installs them. It backs `config profile create`,
// where skills are part of the bundle just like modules.
func (s GenerateService) SelectSkills(base types.SkillsConfig, prompt Prompter) (types.SkillsConfig, error) {
	if len(catalog.AllAgentSkills(domain.SkillDirs()...)) == 0 {
		return types.SkillsConfig{}, nil
	}

	ctx := wizardContext{base: &types.DevcontainerConfig{Skills: base}}
	state, err := prompt.Wizard(ctx.skillSteps)
	if err != nil {
		return types.SkillsConfig{}, err
	}
	return ctx.selectedSkills(state), nil
}
