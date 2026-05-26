package dockerfile

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"
)

var JavaTemurinModule = &DockerfileModuleSpec{
	ID:        "java-temurin",
	Label:     "Java — Eclipse Temurin JDK + Maven",
	Category:  core.CategoryLang,
	Conflicts: []string{"java-openjdk"},
	Options: []core.ModuleOption{
		{
			ID:    "versions",
			Label: "JDK versions",
			Type:  core.ModuleOptionMultiselect,
			Choices: []core.ModuleOptionChoice{
				{Value: "11", Label: "Temurin JDK 11"},
				{Value: "17", Label: "Temurin JDK 17"},
				{Value: "21", Label: "Temurin JDK 21"},
			},
			Default: []string{"17", "21"},
		},
		{
			ID:      "maven",
			Label:   "Install Maven",
			Type:    core.ModuleOptionConfirm,
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
			pkgs = append(pkgs, fmt.Sprintf("temurin-%s-jdk", v))
		}
		if maven {
			pkgs = append(pkgs, "maven")
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
