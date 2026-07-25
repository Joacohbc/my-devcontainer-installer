package domain

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var categoryOrder = []types.DockerfileCategory{
	types.CategoryBase,
	types.CategoryInfra,
	types.CategoryLang,
	types.CategoryRuntime,
	types.CategoryDB,
	types.CategoryCleanup,
}

type ResolvedModule struct {
	Module  *dockerfile.ModuleSpec
	Options map[string]any
}

type ResolverError struct{ error }

func ResolveDockerfileModules(selected []types.SelectedModule) ([]ResolvedModule, error) {
	byID, err := seedSelectedModules(selected)
	if err != nil {
		return nil, err
	}
	if err := resolveRequiredModules(byID); err != nil {
		return nil, err
	}
	if err := assertNoModuleConflicts(byID); err != nil {
		return nil, err
	}
	return orderModulesByCategory(byID), nil
}

func seedSelectedModules(selected []types.SelectedModule) (map[types.ModuleID]map[string]any, error) {
	byID := make(map[types.ModuleID]map[string]any)
	for _, m := range catalog.DockerfileModules {
		if m.Always {
			byID[m.ID] = map[string]any{}
		}
	}
	for _, sel := range selected {
		if catalog.GetDockerfileModule(sel.ID) == nil {
			return nil, ResolverError{fmt.Errorf("Unknown module: %s", sel.ID)}
		}
		options := sel.Options
		if options == nil {
			options = map[string]any{}
		}
		byID[sel.ID] = options
	}
	return byID, nil
}

func resolveRequiredModules(byID map[types.ModuleID]map[string]any) error {
	stack := make([]types.ModuleID, 0, len(byID))
	for id := range byID {
		stack = append(stack, id)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, req := range catalog.GetDockerfileModule(id).Requires {
			if _, exists := byID[req]; exists {
				continue
			}
			if catalog.GetDockerfileModule(req) == nil {
				return ResolverError{fmt.Errorf("Module %s requires unknown module %s", id, req)}
			}
			byID[req] = map[string]any{}
			stack = append(stack, req)
		}
	}
	return nil
}

func assertNoModuleConflicts(byID map[types.ModuleID]map[string]any) error {
	for id := range byID {
		for _, conflict := range catalog.GetDockerfileModule(id).Conflicts {
			if _, exists := byID[conflict]; exists {
				return ResolverError{fmt.Errorf("Module %s conflicts with %s", id, conflict)}
			}
		}
	}
	return nil
}

func orderModulesByCategory(byID map[types.ModuleID]map[string]any) []ResolvedModule {
	resolved := make([]ResolvedModule, 0, len(byID))
	for _, cat := range categoryOrder {
		for _, mod := range catalog.DockerfileModules {
			if mod.Category != cat {
				continue
			}
			options, exists := byID[mod.ID]
			if !exists {
				continue
			}
			resolved = append(resolved, ResolvedModule{Module: mod, Options: options})
		}
	}
	return resolved
}
