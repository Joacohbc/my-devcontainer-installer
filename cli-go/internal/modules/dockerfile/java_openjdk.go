package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var JavaOpenjdkModule = &DockerfileModuleSpec{
	ID:        "java-openjdk",
	Label:     "Java — OpenJDK (Ubuntu repos) + Maven",
	Category:  core.CategoryLang,
	Conflicts: []string{"java-temurin"},
	Options: []core.ModuleOption{
		{
			ID:    "versions",
			Label: "JDK versions",
			Type:  core.ModuleOptionMultiselect,
			Choices: []core.ModuleOptionChoice{
				{Value: "11", Label: "openjdk-11-jdk"},
				{Value: "17", Label: "openjdk-17-jdk"},
				{Value: "21", Label: "openjdk-21-jdk"},
			},
			Default: []string{"17"},
		},
		{
			ID:      "maven",
			Label:   "Install Maven",
			Type:    core.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		versions := stringsFromAny(opts["versions"], []string{"17"})
		maven := true
		if v, ok := opts["maven"]; ok {
			if b, ok := v.(bool); ok {
				maven = b
			}
		}
		pkgs := make([]string, 0, len(versions)+1)
		for _, v := range versions {
			pkgs = append(pkgs, fmt.Sprintf("openjdk-%s-jdk", v))
		}
		if maven {
			pkgs = append(pkgs, "maven")
		}
		return fmt.Sprintf(`##
## JAVA (OpenJDK)
##
RUN apt-get update && apt-get install -y %s
`, strings.Join(pkgs, " "))
	},
}
