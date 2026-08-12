package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var modeLabels = map[types.BuildMode]string{
	types.BuildModeLocalCached: "local-cached — generate Dockerfile + compose, reuse cached image when unchanged",
	types.BuildModeRemote:      "remote — skip the build, pull a pre-built image from the registry",
}

const (
	stepKeyWorkspace    = "workspace"
	stepKeyMode         = "mode"
	stepKeyProfile      = "profile"
	stepKeyVariant      = "variant"
	stepKeyImage        = "image"
	stepKeySubnet       = "subnet"
	stepKeyPorts        = "ports"
	stepKeyVolumes      = "volumes"
	stepKeySharedConfig = "sharedConfig"
)

func categoryStepKey(c types.UICategory) string  { return "cat:" + string(c) }
func optionStepKey(entryID, optID string) string { return "opt:" + entryID + ":" + optID }
func envStepKey(name string) string              { return "env:" + name }

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
		enabled := true
		if v, ok := initial.(bool); ok {
			enabled = v
		}
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

func currentMode(s *State) types.BuildMode {
	if mode := s.String(stepKeyMode); mode != "" {
		return types.BuildMode(mode)
	}
	return types.BuildModeLocalCached
}

func categoryChoices(c types.UICategory, mode types.BuildMode, selectedModules map[string]bool) []Option {
	var choices []Option
	for _, e := range catalog.SelectableByCategory()[c] {
		if !e.IsService && mode != types.BuildModeLocalCached {
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
	userProfiles    []catalog.Profile
}

func (w wizardContext) steps(s *State) []Step {
	steps := []Step{w.workspaceStep(), w.modeStep()}

	if currentMode(s) == types.BuildModeRemote {
		steps = append(steps, w.variantStep())
	} else if step, ok := w.profileStep(); ok {
		// In local-cached mode, optionally start from one of the user's profiles,
		// which pre-selects its modules before the per-category steps.
		steps = append(steps, step)
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
			Title:   "Puertos a publicar en el devcontainer (ej. 8080:80,5432:5432 — bind a 127.0.0.1; vacío para ninguno):",
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
			Title:   "Volúmenes extra a montar en el devcontainer (ej. myvol:/data,./cache:/cache — vacío para ninguno):",
			Initial: initial,
		}
	}}
}

func (w wizardContext) sharedConfigStep() Step {
	return Step{Key: stepKeySharedConfig, Build: func(s *State) Field {
		initial := types.SharedConfigEnabled(w.base)
		if s.Has(stepKeySharedConfig) {
			initial = s.Bool(stepKeySharedConfig)
		}
		return Field{
			Kind:    FieldConfirm,
			Title:   "Montar el volumen global de config compartida (logins/sesiones de Claude, gh, codex… persisten entre contenedores)?",
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

func (w wizardContext) modeStep() Step {
	return Step{Key: stepKeyMode, Build: func(s *State) Field {
		choices := make([]Option, len(types.BuildModes))
		for i, m := range types.BuildModes {
			choices[i] = Option{Value: string(m), Label: modeLabels[m]}
		}
		initial := string(w.base.Mode)
		if initial == "" {
			initial = string(types.BuildModeLocalCached)
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
		return Field{Kind: FieldSelect, Title: "Image variant:", Choices: VariantChoices(), Initial: initial}
	}}
}

// profileStep lets the user start from one of their saved profiles (a module
// bundle). It is only offered in local-cached mode and when at least one user
// profile exists. Selecting one seeds the per-category module multiselects.
func (w wizardContext) profileStep() (Step, bool) {
	if len(w.userProfiles) == 0 {
		return Step{}, false
	}
	return Step{Key: stepKeyProfile, Build: func(s *State) Field {
		choices := []Option{{Value: "", Label: "(ninguno — elegir módulos manualmente)"}}
		for _, p := range w.userProfiles {
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
		return Field{Kind: FieldSelect, Title: "Partir de un perfil? (opcional):", Choices: choices, Initial: initial}
	}}, true
}

func (w wizardContext) profileByID(id string) (catalog.Profile, bool) {
	for _, p := range w.userProfiles {
		if p.ID == id {
			return p, true
		}
	}
	return catalog.Profile{}, false
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
			if pid := s.String(stepKeyProfile); pid != "" {
				if p, ok := w.profileByID(pid); ok {
					initial = profileCategoryIDs(p, category)
				}
			}
			if s.Has(categoryStepKey(category)) {
				initial = s.Strings(categoryStepKey(category))
			}
			return Field{
				Kind:    FieldMultiselect,
				Title:   types.UICategoryLabels[category] + " (Espacio para seleccionar, Enter para confirmar):",
				Choices: categoryChoices(category, currentMode(s), selectedModuleIDs(s)),
				Initial: initial,
			}
		}})
	}
	return steps
}

func (w wizardContext) optionSteps(s *State) []Step {
	selected := selectedEntryIDs(s)
	localCached := currentMode(s) == types.BuildModeLocalCached
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
	return currentMode(s) == types.BuildModeRemote && s.String(stepKeyVariant) != ""
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
	if mode == types.BuildModeLocalCached {
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

	if mode == types.BuildModeRemote {
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
// Remote mode builds no image here, so it carries no scripts either.
func (w wizardContext) selectedScripts(s *State) []types.CustomScript {
	if currentMode(s) != types.BuildModeLocalCached {
		return nil
	}
	pid := s.String(stepKeyProfile)
	if pid == "" {
		return w.base.Dockerfile.Scripts
	}
	p, ok := w.profileByID(pid)
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
		userProfiles:    catalog.LoadUserProfiles(domain.ProfileDirs()...),
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
					Title:   types.UICategoryLabels[category] + " (Espacio para seleccionar, Enter para confirmar):",
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

// VariantChoices lists the selectable remote image variants.
func VariantChoices() []Option {
	choices := make([]Option, len(types.RemoteVariants))
	for i, v := range types.RemoteVariants {
		choices[i] = Option{Value: v, Label: types.VariantLabels[v]}
	}
	return choices
}
