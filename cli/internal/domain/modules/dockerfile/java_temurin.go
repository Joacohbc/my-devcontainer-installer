package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var JavaTemurinModule = &ModuleSpec{
	ID:         "java-temurin",
	Label:      "Java — Eclipse Temurin JDK + Maven",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Conflicts:  []string{"java-openjdk"},
	Options: []types.ModuleOption{
		{
			ID:    "versions",
			Label: "JDK versions",
			Type:  types.ModuleOptionMultiselect,
			Choices: []types.ModuleOptionChoice{
				{Value: "none", Label: "none (No JDK versions)"},
				{Value: "11", Label: "Temurin JDK 11"},
				{Value: "17", Label: "Temurin JDK 17"},
				{Value: "21", Label: "Temurin JDK 21"},
			},
			Default: []string{"17", "21"},
		},
		{
			ID:      "maven",
			Label:   "Install Maven",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		versions := stringsFromAny(opts["versions"], []string{"17", "21"})
		maven := true
		if v, ok := opts["maven"]; ok {
			if b, ok := v.(bool); ok {
				maven = b
			}
		}
		pkgs := make([]string, 0, len(versions)+1)
		for _, v := range versions {
			if v != "none" {
				pkgs = append(pkgs, fmt.Sprintf("temurin-%s-jdk", v))
			}
		}
		if maven {
			pkgs = append(pkgs, "maven")
		}
		if len(pkgs) == 0 {
			return ""
		}
		return fmt.Sprintf(`##
## JAVA (Temurin)
##
RUN wget -O - https://packages.adoptium.net/artifactory/api/gpg/key/public | apt-key add - && \
    echo "deb https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" | tee /etc/apt/sources.list.d/adoptium.list && \
    apt-get update && \
    apt-get install -y %s
`, strings.Join(pkgs, " "))
	},
}
