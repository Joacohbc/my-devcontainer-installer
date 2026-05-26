package domain

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/modules/dockerfile"
	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/registry"
)

var categoryOrder = []core.DockerfileCategory{
	core.CategoryBase,
	core.CategoryInfra,
	core.CategoryLang,
	core.CategoryRuntime,
	core.CategoryDB,
	core.CategoryCleanup,
}

type ResolvedModule struct {
	Module  *dockerfile.DockerfileModuleSpec
	Options map[string]any
}

type ResolverError struct{ error }

func ResolveDockerfileModules(selected []core.SelectedModule) ([]ResolvedModule, error) {
	byID := make(map[string]map[string]any)

	for _, m := range registry.DockerfileModules {
		if m.Always {
			byID[m.ID] = map[string]any{}
		}
	}

	for _, sel := range selected {
		mod := registry.GetDockerfileModule(sel.ID)
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
		mod := registry.GetDockerfileModule(id)
		for _, req := range mod.Requires {
			if _, exists := byID[req]; !exists {
				if registry.GetDockerfileModule(req) == nil {
					return nil, ResolverError{fmt.Errorf("Module %s requires unknown module %s", id, req)}
				}
				byID[req] = map[string]any{}
				stack = append(stack, req)
			}
		}
	}

	for id := range byID {
		mod := registry.GetDockerfileModule(id)
		for _, conflict := range mod.Conflicts {
			if _, exists := byID[conflict]; exists {
				return nil, ResolverError{fmt.Errorf("Module %s conflicts with %s", id, conflict)}
			}
		}
	}

	resolved := make([]ResolvedModule, 0, len(byID))
	for _, cat := range categoryOrder {
		for _, mod := range registry.DockerfileModules {
			if mod.Category == cat {
				if opts, exists := byID[mod.ID]; exists {
					resolved = append(resolved, ResolvedModule{Module: mod, Options: opts})
				}
			}
		}
	}
	return resolved, nil
}
