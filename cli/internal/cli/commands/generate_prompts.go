package commands

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var modeLabels = map[types.BuildMode]string{
	types.BuildModeLocalCached: "local-cached — generate Dockerfile + compose, reuse cached image when unchanged",
	types.BuildModeRemote:      "remote — skip the build, pull a pre-built image from the registry",
}

func choicesFromOption(o types.ModuleOption) []prompt.Choice {
	choices := make([]prompt.Choice, len(o.Choices))
	for i, c := range o.Choices {
		choices[i] = prompt.Choice{Value: c.Value, Label: c.Label}
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

func promptOption(o types.ModuleOption, def any) (any, error) {
	switch o.Type {
	case types.ModuleOptionSelect:
		s, _ := def.(string)
		return prompt.Select("  "+o.Label+":", choicesFromOption(o), s)
	case types.ModuleOptionMultiselect:
		return prompt.Multiselect("  "+o.Label+":", choicesFromOption(o), defaultStrings(def))
	case types.ModuleOptionConfirm:
		b := true
		if v, ok := def.(bool); ok {
			b = v
		}
		return prompt.Confirm("  "+o.Label+"?", b)
	case types.ModuleOptionInput:
		s, _ := def.(string)
		return prompt.Input("  "+o.Label+":", s, nil)
	}
	return nil, nil
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

func serviceOptionsOf(services []any, id string) map[string]any {
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

func buildConfigFromPrompts(base *types.DevcontainerConfig) (*types.DevcontainerConfig, error) {
	cwd, err := currentDir()
	if err != nil {
		return nil, err
	}

	wsDefault := base.Workspace
	if wsDefault == "" {
		wsDefault = domain.SanitizeDockerName(filepath.Base(cwd), "devcontainer")
	}
	workspace, err := prompt.Input(
		"Workspace name (used as prefix for containers, network, volumes):",
		wsDefault,
		func(v string) error {
			if !domain.IsValidDockerName(v) {
				return fmt.Errorf("invalid Docker name (allowed: a-z A-Z 0-9 _ . -, ≤63 chars, start alphanumeric)")
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	base.Workspace = workspace

	modeChoices := make([]prompt.Choice, len(types.BuildModes))
	for i, m := range types.BuildModes {
		modeChoices[i] = prompt.Choice{Value: string(m), Label: modeLabels[m]}
	}
	modeDefault := string(base.Mode)
	if modeDefault == "" {
		modeDefault = string(types.BuildModeLocalCached)
	}
	modeStr, err := prompt.Select("Build mode:", modeChoices, modeDefault)
	if err != nil {
		return nil, err
	}
	mode := types.BuildMode(modeStr)

	var variant string
	if mode == types.BuildModeRemote {
		variantDefault := "ssh"
		if base.Remote != nil && base.Remote.Variant != "" {
			variantDefault = base.Remote.Variant
		}
		variant, err = prompt.Select("Image variant:", variantChoices(), variantDefault)
		if err != nil {
			return nil, err
		}
	}

	modules := []types.SelectedModule{}
	if mode == types.BuildModeLocalCached {
		var selectable []prompt.Choice
		for _, m := range catalog.DockerfileModules {
			if !m.Always {
				selectable = append(selectable, prompt.Choice{Value: m.ID, Label: m.Label})
			}
		}
		var prevIDs []string
		for _, m := range base.Dockerfile.Modules {
			prevIDs = append(prevIDs, m.ID)
		}
		selectedIDs, merr := prompt.Multiselect("Select Dockerfile modules (Space to select, Enter to confirm):", selectable, prevIDs)
		if merr != nil {
			return nil, merr
		}
		for _, id := range selectedIDs {
			mod := catalog.GetDockerfileModule(id)
			if mod == nil {
				continue
			}
			opts := map[string]any{}
			for _, o := range mod.Options {
				val, oerr := promptOption(o, o.Default)
				if oerr != nil {
					return nil, oerr
				}
				opts[o.ID] = val
			}
			modules = append(modules, types.SelectedModule{ID: id, Options: opts})
		}
	}

	selectedModuleIDs := map[string]bool{}
	for _, m := range modules {
		selectedModuleIDs[m.ID] = true
	}

	var services []any

	// Always-on services may still expose options.
	for _, svc := range catalog.ComposeServices {
		if !svc.Always || len(svc.Options) == 0 {
			continue
		}
		prevOpts := serviceOptionsOf(base.Compose.Services, svc.ID)
		opts := map[string]any{}
		for _, o := range svc.Options {
			if o.RequiresModule != "" && !selectedModuleIDs[o.RequiresModule] {
				continue
			}
			val, oerr := promptOption(o, optionDefault(o, prevOpts))
			if oerr != nil {
				return nil, oerr
			}
			opts[o.ID] = val
		}
		services = append(services, types.SelectedModule{ID: svc.ID, Options: opts})
	}

	var selectableServices []prompt.Choice
	for _, s := range catalog.ComposeServices {
		if s.Always || s.Internal {
			continue
		}
		if s.RequiresModule != "" && !selectedModuleIDs[s.RequiresModule] {
			continue
		}
		selectableServices = append(selectableServices, prompt.Choice{Value: s.ID, Label: s.Label})
	}
	var baseServiceIDs []string
	for _, s := range base.Compose.Services {
		baseServiceIDs = append(baseServiceIDs, serviceIDOf(s))
	}
	serviceIDs, err := prompt.Multiselect("Select compose services:", selectableServices, baseServiceIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range serviceIDs {
		svc := catalog.GetComposeService(id)
		if svc == nil {
			continue
		}
		prevOpts := serviceOptionsOf(base.Compose.Services, id)
		opts := map[string]any{}
		for _, o := range svc.Options {
			val, oerr := promptOption(o, optionDefault(o, prevOpts))
			if oerr != nil {
				return nil, oerr
			}
			opts[o.ID] = val
		}
		services = append(services, types.SelectedModule{ID: id, Options: opts})
	}

	var image string
	if mode == types.BuildModeRemote && variant != "" {
		perProject := ""
		if base.Remote != nil {
			perProject = base.Remote.Registry
		}
		image = domain.ResolveRemoteImage(variant, domain.ResolveRegistry("", perProject))
	} else {
		image, err = prompt.Input("Image name:", base.Image, func(v string) error {
			if !domain.IsValidImageName(v) {
				return fmt.Errorf("invalid Docker image name")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	usedSubnets := domain.ListUsedSubnets(captureFunc())
	preferredSubnet := base.Compose.Subnet
	if preferredSubnet == "" {
		preferredSubnet = domain.DefaultSubnet
	}
	suggestedSubnet := domain.FindFreeSubnet(preferredSubnet, usedSubnets)
	if suggestedSubnet != preferredSubnet {
		color.Yellow("Subnet %s overlaps with existing Docker network. Suggesting %s.", preferredSubnet, suggestedSubnet)
	}
	subnet, err := prompt.Input("Docker network subnet (CIDR):", suggestedSubnet, func(v string) error {
		if !domain.IsValidCidr(v) {
			return fmt.Errorf("invalid CIDR")
		}
		if clash := domain.SubnetConflict(v, usedSubnets); clash != nil {
			return fmt.Errorf("overlaps with existing network %s", domain.FormatCidr(*clash))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for k, v := range base.Env {
		env[k] = v
	}
	draft := &types.DevcontainerConfig{
		Mode:       mode,
		Image:      image,
		Workspace:  workspace,
		Dockerfile: types.DockerfileConfig{Modules: modules},
		Compose:    types.ComposeConfig{Services: services, Subnet: subnet},
		Env:        env,
	}
	if mode == types.BuildModeRemote && variant != "" {
		perProject := ""
		if base.Remote != nil {
			perProject = base.Remote.Registry
		}
		draft.Remote = &types.RemoteConfig{Variant: variant, Registry: perProject}
	}
	for _, e := range domain.CollectRequiredEnvVars(draft) {
		cur := env[e.Name]
		if cur == "" {
			cur = e.Default
		}
		val, eerr := prompt.Input(e.Prompt+":", cur, nil)
		if eerr != nil {
			return nil, eerr
		}
		env[e.Name] = val
	}
	draft.Env = env
	return draft, nil
}

func variantChoices() []prompt.Choice {
	choices := make([]prompt.Choice, len(types.RemoteVariants))
	for i, v := range types.RemoteVariants {
		choices[i] = prompt.Choice{Value: v, Label: types.VariantLabels[v]}
	}
	return choices
}
