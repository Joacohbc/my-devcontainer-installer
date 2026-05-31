package service

import (
	"fmt"
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var modeLabels = map[types.BuildMode]string{
	types.BuildModeLocalCached: "local-cached — generate Dockerfile + compose, reuse cached image when unchanged",
	types.BuildModeRemote:      "remote — skip the build, pull a pre-built image from the registry",
}

const (
	stepKeyWorkspace = "workspace"
	stepKeyMode      = "mode"
	stepKeyVariant   = "variant"
	stepKeyImage     = "image"
	stepKeySubnet    = "subnet"
	stepKeyPersist   = "persist"
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
	switch d := v.(type) {
	case []string:
		return d
	case []any:
		var out []string
		for _, x := range d {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func serviceIDOf(s any) string {
	switch v := s.(type) {
	case string:
		return v
	case types.SelectedModule:
		return v.ID
	case map[string]any:
		if id, ok := v["id"].(string); ok {
			return id
		}
	}
	return ""
}

// ServiceOptionsOf returns the stored options for the compose service id within
// a config's service list.
func ServiceOptionsOf(services []any, id string) map[string]any {
	for _, s := range services {
		if serviceIDOf(s) != id {
			continue
		}
		switch v := s.(type) {
		case types.SelectedModule:
			return v.Options
		case map[string]any:
			if opts, ok := v["options"].(map[string]any); ok {
				return opts
			}
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
		if m.ID == id {
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
			if svc := catalog.GetComposeService(e.ID); svc != nil && svc.RequiresModule != "" && !selectedModules[svc.RequiresModule] {
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
		if catalog.GetDockerfileModule(id) != nil {
			set[id] = true
		}
	}
	return set
}

func baseCategoryIDs(base *types.DevcontainerConfig, c types.UICategory) []string {
	var ids []string
	for _, m := range base.Dockerfile.Modules {
		if spec := catalog.GetDockerfileModule(m.ID); spec != nil && spec.UICategory == c {
			ids = append(ids, m.ID)
		}
	}
	for _, s := range base.Compose.Services {
		id := serviceIDOf(s)
		if spec := catalog.GetComposeService(id); spec != nil && spec.UICategory == c {
			ids = append(ids, id)
		}
	}
	return ids
}

type wizardContext struct {
	base            *types.DevcontainerConfig
	workspace       string
	usedSubnets     []domain.CidrRange
	suggestedSubnet string
}

func (w wizardContext) steps(s *State) []Step {
	steps := []Step{w.workspaceStep(), w.modeStep()}

	if currentMode(s) == types.BuildModeRemote {
		steps = append(steps, w.variantStep())
	}

	steps = append(steps, w.categorySteps(s)...)
	steps = append(steps, w.optionSteps(s)...)

	if !w.remoteImageDerived(s) {
		steps = append(steps, w.imageStep())
	}
	steps = append(steps, w.subnetStep())
	steps = append(steps, w.persistStep())
	steps = append(steps, w.envSteps(s)...)
	return steps
}

func basePersistIDs(base *types.DevcontainerConfig) []string {
	if base.Compose.PersistVolumes == nil {
		return types.DefaultPersistVolumeIDs()
	}
	return *base.Compose.PersistVolumes
}

func (w wizardContext) persistStep() Step {
	return Step{Key: stepKeyPersist, Build: func(s *State) Field {
		initial := basePersistIDs(w.base)
		if s.Has(stepKeyPersist) {
			initial = s.Strings(stepKeyPersist)
		}
		choices := make([]Option, len(types.PersistVolumeSpecs))
		for i, spec := range types.PersistVolumeSpecs {
			choices[i] = Option{Value: spec.ID, Label: spec.Label}
		}
		return Field{
			Kind:    FieldMultiselect,
			Title:   "Volúmenes persistentes a montar (Espacio para seleccionar, Enter para confirmar):",
			Choices: choices,
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

func (w wizardContext) categorySteps(s *State) []Step {
	var steps []Step
	for _, c := range catalog.CategoriesInOrder() {
		if len(categoryChoices(c, currentMode(s), selectedModuleIDs(s))) == 0 {
			continue
		}
		category := c
		steps = append(steps, Step{Key: categoryStepKey(category), Build: func(s *State) Field {
			initial := baseCategoryIDs(w.base, category)
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
		if !localCached || !selected[m.ID] {
			continue
		}
		steps = append(steps, w.entryOptionSteps(m.ID, m.Options, moduleOptionsOf(w.base.Dockerfile.Modules, m.ID))...)
	}
	for _, svc := range catalog.ComposeServices {
		if !selected[svc.ID] {
			continue
		}
		steps = append(steps, w.entryOptionSteps(svc.ID, svc.Options, ServiceOptionsOf(w.base.Compose.Services, svc.ID))...)
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
			if !selected[m.ID] {
				continue
			}
			modules = append(modules, types.SelectedModule{ID: m.ID, Options: w.entryOptions(s, m.ID, m.Options, moduleOptionsOf(w.base.Dockerfile.Modules, m.ID))})
		}
	}

	var services []any
	for _, svc := range catalog.ComposeServices {
		if !selected[svc.ID] {
			continue
		}
		services = append(services, types.SelectedModule{ID: svc.ID, Options: w.entryOptions(s, svc.ID, svc.Options, ServiceOptionsOf(w.base.Compose.Services, svc.ID))})
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
		Dockerfile: types.DockerfileConfig{Modules: modules},
		Compose:    types.ComposeConfig{Services: services, Subnet: s.String(stepKeySubnet)},
		Env:        env,
	}

	if s.Has(stepKeyPersist) {
		persist := s.Strings(stepKeyPersist)
		if persist == nil {
			persist = []string{}
		}
		draft.Compose.PersistVolumes = &persist
	} else {
		draft.Compose.PersistVolumes = w.base.Compose.PersistVolumes
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

	usedSubnets := domain.ListUsedSubnets(captureFunc())
	preferredSubnet := base.Compose.Subnet
	if preferredSubnet == "" {
		preferredSubnet = domain.DefaultSubnet
	}

	ctx := wizardContext{
		base:            base,
		workspace:       workspace,
		usedSubnets:     usedSubnets,
		suggestedSubnet: domain.FindFreeSubnet(preferredSubnet, usedSubnets),
	}

	state, err := prompt.Wizard(ctx.steps)
	if err != nil {
		return nil, err
	}
	return ctx.reduce(state), nil
}

// VariantChoices lists the selectable remote image variants.
func VariantChoices() []Option {
	choices := make([]Option, len(types.RemoteVariants))
	for i, v := range types.RemoteVariants {
		choices[i] = Option{Value: v, Label: types.VariantLabels[v]}
	}
	return choices
}
