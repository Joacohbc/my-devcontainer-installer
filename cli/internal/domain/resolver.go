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
	byID := make(map[string]map[string]any)

	for _, m := range catalog.DockerfileModules {
		if m.Always {
			byID[m.ID] = map[string]any{}
		}
	}

	for _, sel := range selected {
		mod := catalog.GetDockerfileModule(sel.ID)
		if mod == nil {
			return nil, ResolverError{fmt.Errorf("Unknown module: %s", sel.ID)}
		}
		opts := sel.Options
		if opts == nil {
			opts = map[string]any{}
		}
		byID[sel.ID] = opts
	}

	stack := make([]string, 0, len(byID))
	for id := range byID {
		stack = append(stack, id)
	}

	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		mod := catalog.GetDockerfileModule(id)
		for _, req := range mod.Requires {
			if _, exists := byID[req]; !exists {
				if catalog.GetDockerfileModule(req) == nil {
					return nil, ResolverError{fmt.Errorf("Module %s requires unknown module %s", id, req)}
				}
				byID[req] = map[string]any{}
				stack = append(stack, req)
			}
		}
	}

	for id := range byID {
		mod := catalog.GetDockerfileModule(id)
		for _, conflict := range mod.Conflicts {
			if _, exists := byID[conflict]; exists {
				return nil, ResolverError{fmt.Errorf("Module %s conflicts with %s", id, conflict)}
			}
		}
	}

	resolved := make([]ResolvedModule, 0, len(byID))
	for _, cat := range categoryOrder {
		for _, mod := range catalog.DockerfileModules {
			if mod.Category == cat {
				if opts, exists := byID[mod.ID]; exists {
					resolved = append(resolved, ResolvedModule{Module: mod, Options: opts})
				}
			}
		}
	}
	return resolved, nil
}
