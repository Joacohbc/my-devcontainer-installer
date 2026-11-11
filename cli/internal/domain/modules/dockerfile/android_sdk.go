package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// androidCmdlineToolsBuild pins Google's command-line tools archive so the
// layer is reproducible; sdkmanager installs everything else from it.
const androidCmdlineToolsBuild = "11076708"

const androidSdkRoot = "/opt/android-sdk"

var AndroidSdkModule = &ModuleSpec{
	ID:         types.ModuleAndroidSdk,
	Label:      "Android SDK (cmdline-tools, platform-tools/adb, platform + build-tools)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryDevTools,
	Requires:   []types.ModuleID{types.ModuleJavaTemurin},
	Options: []types.ModuleOption{
		{
			ID:    "api",
			Label: "Android API level (platform and build-tools)",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "34", Label: "Android 14 (API 34)"},
				{Value: "35", Label: "Android 15 (API 35)"},
			},
			Default: "35",
		},
	},
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{
			Assignments: []EnvVar{
				{Name: "ANDROID_HOME", Value: androidSdkRoot},
				{Name: "ANDROID_SDK_ROOT", Value: androidSdkRoot},
			},
			PathEntries: []PathEntry{androidSdkRoot + "/cmdline-tools/latest/bin", androidSdkRoot + "/platform-tools"},
		}
	},
	Context: func(opts map[string]any) *types.ContextSection {
		api := types.StringOpt(opts, "api", "35")
		return &types.ContextSection{
			Title: "Android SDK",
			Body: ctxBody(
				fmt.Sprintf("The Android SDK lives in `%s` (`$ANDROID_HOME`) with platform API %s, its build-tools, and platform-tools (`adb`, `fastboot`).", androidSdkRoot, api),
				"Install more packages with `sdkmanager \"<package>\"`; licenses are already accepted.",
				"There is no emulator and no USB passthrough: reach a device with `adb connect <host>:<port>` (wireless debugging).",
			),
		}
	},
	Render: func(opts map[string]any) string {
		api := types.StringOpt(opts, "api", "35")
		return fmt.Sprintf(`##
## ANDROID SDK
##
RUN apt-get update && apt-get install -y --no-install-recommends unzip && \
    mkdir -p %[1]s/cmdline-tools && \
    wget -qO /tmp/cmdline-tools.zip https://dl.google.com/android/repository/commandlinetools-linux-%[2]s_latest.zip && \
    unzip -q /tmp/cmdline-tools.zip -d %[1]s/cmdline-tools && \
    mv %[1]s/cmdline-tools/cmdline-tools %[1]s/cmdline-tools/latest && \
    rm /tmp/cmdline-tools.zip && \
    yes | %[1]s/cmdline-tools/latest/bin/sdkmanager --licenses > /dev/null && \
    %[1]s/cmdline-tools/latest/bin/sdkmanager --install "platform-tools" "platforms;android-%[3]s" "build-tools;%[3]s.0.0" > /dev/null && \
    chown -R devuser:devuser %[1]s && \
    %[4]s
`, androidSdkRoot, androidCmdlineToolsBuild, api, aptCleanup())
	},
}
