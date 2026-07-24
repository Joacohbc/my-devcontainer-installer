package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var JavaOpenjdkModule = &ModuleSpec{
	ID:         types.ModuleJavaOpenjdk,
	Label:      "Java — OpenJDK (Ubuntu repos) + Maven",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Conflicts:  []types.ModuleID{types.ModuleJavaTemurin},
	Options: []types.ModuleOption{
		{
			ID:    "versions",
			Label: "JDK versions",
			Type:  types.ModuleOptionMultiselect,
			Choices: []types.ModuleOptionChoice{
				{Value: "none", Label: "none (No JDK versions)"},
				{Value: "11", Label: "openjdk-11-jdk"},
				{Value: "17", Label: "openjdk-17-jdk"},
				{Value: "21", Label: "openjdk-21-jdk"},
			},
			Default: []string{"17"},
		},
		{
			ID:      "maven",
			Label:   "Install Maven",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		versions := types.StringsOpt(opts, "versions", []string{"17"})
		maven := types.BoolOpt(opts, "maven", true)
		pkgs := make([]string, 0, len(versions)+1)
		for _, v := range versions {
			if v != "none" {
				pkgs = append(pkgs, fmt.Sprintf("openjdk-%s-jdk", v))
			}
		}
		if maven {
			pkgs = append(pkgs, "maven")
		}
		if len(pkgs) == 0 {
			return ""
		}
		return fmt.Sprintf(`##
## JAVA (OpenJDK)
##
RUN apt-get update && apt-get install -y %s && %s
`, strings.Join(pkgs, " "), aptCleanup())
	},
}
