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
				{Value: "11", Label: "openjdk-11-jdk-headless"},
				{Value: "17", Label: "openjdk-17-jdk-headless"},
				{Value: "21", Label: "openjdk-21-jdk-headless"},
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
	Context: func(opts map[string]any) *types.ContextSection {
		versions := types.StringsOpt(opts, "versions", []string{"17"})
		installed := make([]string, 0, len(versions))
		for _, v := range versions {
			if v != "none" {
				installed = append(installed, "JDK "+v)
			}
		}
		if len(installed) == 0 {
			return nil
		}
		lines := []string{
			"OpenJDK from the Ubuntu repositories, with " + strings.Join(installed, " and ") + " installed.",
			"Switch between them with `sudo update-alternatives --config java`.",
			"",
			"The JDKs are the headless builds: there is no display, so the AWT/Swing",
			"desktop libraries are left out. Headless AWT (image processing, fonts)",
			"still works.",
		}
		if types.BoolOpt(opts, "maven", true) {
			lines = append(lines, "", "Maven (`mvn`) is installed. Gradle is not — use the project's `./gradlew`.")
		} else {
			lines = append(lines, "", "Neither Maven nor Gradle is installed; use the project's own wrapper.")
		}
		return &types.ContextSection{Title: "Java", Body: ctxBody(lines...)}
	},
	Render: func(opts map[string]any) string {
		versions := types.StringsOpt(opts, "versions", []string{"17"})
		maven := types.BoolOpt(opts, "maven", true)
		pkgs := make([]string, 0, len(versions)+1)
		for _, v := range versions {
			if v != "none" {
				pkgs = append(pkgs, fmt.Sprintf("openjdk-%s-jdk-headless", v))
			}
		}
		if maven {
			pkgs = append(pkgs, "maven")
		}
		if len(pkgs) == 0 {
			return ""
		}
		// The -headless packages are the JDK without the desktop toolkit: the
		// full openjdk-N-jdk pulls in X11, GTK and ALSA (~290 MB per version)
		// for windows a container has nowhere to draw.
		return fmt.Sprintf(`##
## JAVA (OpenJDK)
##
RUN apt-get update && apt-get install -y --no-install-recommends %s && %s
`, strings.Join(pkgs, " "), aptCleanup())
	},
}
