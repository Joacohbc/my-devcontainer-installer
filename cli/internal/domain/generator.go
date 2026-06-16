package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

const DefaultSubnet = "172.25.0.0/28"

func GenerateDockerfile(config *types.DevcontainerConfig) (string, error) {
	if config.Mode == types.BuildModeRemote {
		return "", nil
	}
	resolved, err := ResolveDockerfileModules(MatchDBClientVersions(config))
	if err != nil {
		return "", err
	}
	fragments := make([]string, 0, len(resolved)+2)
	for _, r := range resolved {
		fragments = append(fragments, strings.TrimSpace(r.Module.Render(r.Options)))
	}
	postScripts, err := postScriptsDockerfileBlock(config)
	if err != nil {
		return "", err
	}
	if postScripts != "" {
		fragments = append(fragments, postScripts)
	}
	fragments = append(fragments, types.DockerfileLabelBlock(config))
	return types.GeneratedHeader + "\n\n" + strings.Join(fragments, "\n\n") + "\n", nil
}

func postScriptsDockerfileBlock(config *types.DevcontainerConfig) (string, error) {
	manual, autoStart, err := collectPartitionedPostScripts(config)
	if err != nil {
		return "", err
	}
	if len(manual) == 0 && len(autoStart) == 0 {
		return "", nil
	}

	var lines []string
	lines = append(lines, "##", "## POST-INSTALL SCRIPTS", "##")

	var chmodTargets []string
	if len(manual) > 0 {
		// Interactive / manual scripts: copied verbatim, not auto-run.
		lines = append(lines, fmt.Sprintf("COPY %s %s/", strings.Join(manual, " "), types.PostScriptDir))
		chmodTargets = append(chmodTargets, types.PostScriptDir+"/*.sh")
	}
	var symlinks []string
	if len(autoStart) > 0 {
		// Non-interactive installers the entrypoint auto-runs on start. The
		// numeric "NN-" prefix encodes run order so the entrypoint's sorted glob
		// runs agents first and the agent-wiring tools (graphify/caveman) last.
		for _, e := range autoStart {
			lines = append(lines, fmt.Sprintf("COPY %s %s/%02d-%s", e.File, types.PostScriptStartDir, e.Order, e.File))
			// Also expose each auto-start script under its plain name in
			// ~/post-script/ (a relative symlink into start.d/) so the user can
			// still run it manually, exactly like the interactive scripts.
			symlinks = append(symlinks, fmt.Sprintf("ln -sfn start.d/%02d-%s %s/%s", e.Order, e.File, types.PostScriptDir, e.File))
		}
		chmodTargets = append(chmodTargets, types.PostScriptStartDir+"/*.sh")
	}

	run := fmt.Sprintf("RUN chown -R devuser:devuser %s && chmod +x %s",
		types.PostScriptDir, strings.Join(chmodTargets, " "))
	for _, s := range symlinks {
		run += " \\\n    && " + s
	}
	if len(symlinks) > 0 {
		// Own the symlinks themselves by devuser (the targets are already owned
		// via the recursive chown above; -h keeps chown from dereferencing).
		run += " \\\n    && chown -h devuser:devuser " + types.PostScriptDir + "/*.sh"
	}
	lines = append(lines, run)
	return strings.Join(lines, "\n"), nil
}

// postScriptStart pairs an auto-start script with its resolved run order.
type postScriptStart struct {
	File  string
	Order int
}

// collectPartitionedPostScripts splits the required post-scripts into manual
// (interactive, left under PostScriptDir) and auto-start (non-interactive
// installers the entrypoint runs on start), with the auto-start set sorted by
// run order then filename for deterministic output.
func collectPartitionedPostScripts(config *types.DevcontainerConfig) (manual []string, autoStart []postScriptStart, err error) {
	resolved, err := ResolveDockerfileModules(config.Dockerfile.Modules)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[string]bool)
	for _, r := range resolved {
		if r.Module.PostScriptFiles == nil {
			continue
		}
		for _, f := range r.Module.PostScriptFiles(r.Options) {
			if seen[f] {
				continue
			}
			seen[f] = true
			if r.Module.PostScriptAutoStart {
				order := r.Module.PostScriptStartOrder
				if order == 0 {
					order = types.DefaultPostScriptStartOrder
				}
				autoStart = append(autoStart, postScriptStart{File: f, Order: order})
			} else {
				manual = append(manual, f)
			}
		}
	}
	sort.Slice(autoStart, func(i, j int) bool {
		if autoStart[i].Order != autoStart[j].Order {
			return autoStart[i].Order < autoStart[j].Order
		}
		return autoStart[i].File < autoStart[j].File
	})
	return manual, autoStart, nil
}

func ResolveRemoteImage(variant, registry string) string {
	if registry == "" {
		registry = DefaultRegistry
	}
	var suffix string
	if variant == "ssh" {
		suffix = "devcontainer-ssh"
	} else {
		suffix = "devcontainer-" + variant
	}
	return registry + suffix + ":latest"
}

func ResolveDevcontainerImageName(config *types.DevcontainerConfig) string {
	if config.Mode == types.BuildModeRemote && config.Remote != nil {
		reg := config.Remote.Registry
		return ResolveRemoteImage(config.Remote.Variant, reg)
	}
	if config.Mode == types.BuildModeLocalCached && config.Fingerprint != "" {
		fp := config.Fingerprint
		if len(fp) > 12 {
			fp = fp[:12]
		}
		return types.ImageNamespace + "/" + fp + ":latest"
	}
	return config.Image
}

// devcontainerBuild returns the compose `build:` block for a local-cached
// devcontainer. It bakes the host UID/GID (resolved by the service layer) as
// build args so the image's devuser owns the bind-mounted workspace without a
// runtime remap. With no host ids resolved it falls back to a context-only
// build (the Dockerfile's ARG defaults apply).
func devcontainerBuild(config *types.DevcontainerConfig) *compose.BuildDef {
	build := &compose.BuildDef{Context: "."}
	if config.BuildUID > 0 && config.BuildGID > 0 {
		build.Args = map[string]string{
			"USER_UID": strconv.Itoa(config.BuildUID),
			"USER_GID": strconv.Itoa(config.BuildGID),
		}
	}
	return build
}

// devcontainerPorts returns the published port mappings for the devcontainer
// service, with each spec bound to 127.0.0.1 unless it already carries an
// explicit host IP. Returns nil when no ports are configured.
func devcontainerPorts(config *types.DevcontainerConfig) []string {
	if len(config.Compose.Ports) == 0 {
		return nil
	}
	ports := make([]string, len(config.Compose.Ports))
	for i, p := range config.Compose.Ports {
		ports[i] = BindLoopback(p)
	}
	return ports
}

func prefixContainer(workspace, base string) string {
	if base == workspace || strings.HasPrefix(base, workspace+"-") {
		return base
	}
	return workspace + "-" + base
}

func prefixVolume(workspace, base string) string {
	if strings.HasPrefix(base, workspace+"_") {
		return base
	}
	return workspace + "_" + base
}

// userNamedVolume reports whether a user-supplied mount spec ("src:dst[:opts]")
// references a named volume (as opposed to a host bind mount). It returns the
// volume name when so. Sources beginning with '/', '.', '~' or '$' are treated
// as host paths and need no top-level declaration.
func userNamedVolume(mount string) (string, bool) {
	colon := strings.Index(mount, ":")
	if colon <= 0 {
		return "", false
	}
	src := mount[:colon]
	switch src[0] {
	case '/', '.', '~', '$':
		return "", false
	}
	return src, true
}

func remapVolumeMount(workspace string, declared map[string]bool, mount string) string {
	colon := strings.Index(mount, ":")
	if colon <= 0 {
		return mount
	}
	src := mount[:colon]
	if !declared[src] {
		return mount
	}
	return prefixVolume(workspace, src) + mount[colon:]
}

type enabledServicesResult struct {
	enabledIDs  []string
	optionsByID map[string]map[string]any
}

func resolveEnabledServices(config *types.DevcontainerConfig) enabledServicesResult {
	selected := types.NormalizeServices(config.Compose.Services)
	enabled := make(map[string]bool)
	optionsByID := make(map[string]map[string]any)

	for _, s := range selected {
		enabled[string(s.ID)] = true
		opts := s.Options
		if opts == nil {
			opts = map[string]any{}
		}
		optionsByID[string(s.ID)] = opts
	}

	for _, svc := range catalog.ComposeServices {
		if svc.Always {
			enabled[string(svc.ID)] = true
		}
	}

	enabledIDs := make([]string, 0, len(enabled))
	for id := range enabled {
		enabledIDs = append(enabledIDs, id)
	}
	return enabledServicesResult{enabledIDs: enabledIDs, optionsByID: optionsByID}
}

func GenerateCompose(config *types.DevcontainerConfig) (string, error) {
	result := resolveEnabledServices(config)
	enabledIDs := result.enabledIDs
	optionsByID := result.optionsByID

	workspace := config.Workspace
	networkName := workspace + "-network"

	mapNetwork := func(n string) string {
		switch n {
		case "local-network":
			return networkName
		default:
			return n
		}
	}

	labels := types.ComposeLabels(config)
	subnet := config.Compose.Subnet
	if subnet == "" {
		subnet = DefaultSubnet
	}
	devcontainerIP, _ := LastHost(subnet)

	declaredVolumes := make(map[string]bool)
	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return "", fmt.Errorf("unknown compose service: %s", id)
		}
		for _, v := range svc.Volumes {
			declaredVolumes[v] = true
		}
	}

	devcontainerImageName := ResolveDevcontainerImageName(config)
	services := make(map[string]*compose.ServiceDef)

	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return "", fmt.Errorf("unknown compose service: %s", id)
		}

		imageNameForSvc := config.Image
		if svc.ID == types.ServiceDevcontainer {
			imageNameForSvc = devcontainerImageName
		}

		opts := optionsByID[id]
		if opts == nil {
			opts = map[string]any{}
		}

		dbUser, dbPass := ResolveDBCredentials()
		rc := compose.RenderContext{
			ImageName:         imageNameForSvc,
			EnabledServiceIDs: enabledIDs,
			Options:           opts,
			DefaultDBUser:     dbUser,
			DefaultDBPassword: dbPass,
		}
		if svc.ID == types.ServiceDevcontainer {
			rc.Ports = devcontainerPorts(config)
			rc.WorkspaceDir = types.WorkspaceDir(workspace)
			if types.SharedConfigEnabled(config) {
				rc.SharedConfigMount = types.SharedConfigMount()
			}
		}
		rendered := svc.Render(rc)
		if rendered == nil {
			continue
		}

		if svc.ID == types.ServiceDevcontainer {
			hasDod := false
			for _, m := range config.Dockerfile.Modules {
				if m.ID == types.ModuleDod {
					hasDod = true
					break
				}
			}
			if hasDod {
				rendered.Volumes = append(rendered.Volumes, "/var/run/docker.sock:/var/run/docker.sock")
			}
			// User-defined extra mounts are appended verbatim; named-volume
			// sources are declared in the top-level volumes section below.
			rendered.Volumes = append(rendered.Volumes, config.Compose.Volumes...)
		}

		if svc.ID == types.ServiceDevcontainer {
			switch config.Mode {
			case types.BuildModeRemote:
				rendered.Build = nil
			default:
				rendered.Build = devcontainerBuild(config)
			}
		}

		if svc.ID == types.ServiceDevcontainer {
			rendered.Hostname = workspace
		}

		baseContainer := rendered.ContainerName
		if baseContainer == "" {
			baseContainer = string(svc.ID)
		}
		rendered.ContainerName = prefixContainer(workspace, baseContainer)

		remappedVolumes := make([]string, len(rendered.Volumes))
		for i, v := range rendered.Volumes {
			remappedVolumes[i] = remapVolumeMount(workspace, declaredVolumes, v)
		}
		rendered.Volumes = remappedVolumes

		if networksSlice, ok := rendered.Networks.([]string); ok {
			mapped := make([]string, len(networksSlice))
			for i, n := range networksSlice {
				mapped[i] = mapNetwork(n)
			}
			if svc.ID == types.ServiceDevcontainer && devcontainerIP != "" {
				netObj := make(map[string]any, len(mapped))
				for _, n := range mapped {
					if n == networkName {
						netObj[n] = map[string]any{
							"ipv4_address": fmt.Sprintf("${DEVCONTAINER_IP:-%s}", devcontainerIP),
						}
					} else {
						netObj[n] = nil
					}
				}
				rendered.Networks = netObj
			} else {
				rendered.Networks = mapped
			}
		}

		rendered.Labels = copyLabels(labels)

		serviceKey := string(svc.ID)
		if svc.ID == types.ServiceDevcontainer {
			serviceKey = compose.SSHServiceName
		}
		services[serviceKey] = rendered
	}

	volumes := make(map[string]*compose.VolumeDef)
	for v := range declaredVolumes {
		volumes[prefixVolume(workspace, v)] = &compose.VolumeDef{Labels: copyLabels(labels)}
	}
	// Declare named-volume sources from the user's extra mounts so compose does
	// not reject them as undefined. Bind-mount sources (host paths) need no
	// declaration. Names are kept verbatim (compose prefixes them by project).
	for _, mount := range config.Compose.Volumes {
		if name, ok := userNamedVolume(mount); ok {
			if _, exists := volumes[name]; !exists {
				volumes[name] = &compose.VolumeDef{Labels: copyLabels(labels)}
			}
		}
	}
	// The shared tool-config volume is daemon-level (shared by every workspace),
	// so it is declared external (never prefixed, never removed by compose down -v).
	if types.SharedConfigEnabled(config) {
		volumes[types.SharedConfigVolumeName] = &compose.VolumeDef{External: true}
	}

	networks := make(map[string]*compose.NetworkDef)
	networks[networkName] = &compose.NetworkDef{
		Driver: "bridge",
		IPAM: &compose.IPAMConfig{
			Config: []compose.IPAMSubnet{
				{Subnet: fmt.Sprintf("${DOCKER_SUBNET:-%s}", subnet)},
			},
		},
		Labels: copyLabels(labels),
	}

	doc := compose.ComposeDoc{
		Name:     workspace,
		Services: services,
		Networks: networks,
		Volumes:  volumes,
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return types.GeneratedHeader + "\n" + string(out), nil
}

func GenerateEnv(config *types.DevcontainerConfig) string {
	env := make(map[string]string, len(config.Env))
	for k, v := range config.Env {
		env[k] = v
	}
	if _, ok := env["DOCKER_SUBNET"]; !ok && config.Compose.Subnet != "" {
		env["DOCKER_SUBNET"] = config.Compose.Subnet
	}
	if _, ok := env["DEVCONTAINER_IP"]; !ok {
		effectiveSubnet := env["DOCKER_SUBNET"]
		if effectiveSubnet == "" {
			effectiveSubnet = config.Compose.Subnet
		}
		if effectiveSubnet == "" {
			effectiveSubnet = DefaultSubnet
		}
		ip, ok := LastHost(effectiveSubnet)
		if ok {
			env["DEVCONTAINER_IP"] = ip
		}
	}
	if _, ok := env["COMPOSE_PROJECT_NAME"]; !ok && config.Workspace != "" {
		env["COMPOSE_PROJECT_NAME"] = config.Workspace
	}

	lines := []string{"# Generated by devcontainer CLI"}
	for k, v := range env {
		lines = append(lines, k+"="+v)
	}
	return strings.Join(lines, "\n") + "\n"
}

func CollectRequiredCopyFiles(config *types.DevcontainerConfig) ([]string, error) {
	resolved, err := ResolveDockerfileModules(config.Dockerfile.Modules)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var files []string
	for _, r := range resolved {
		for _, f := range r.Module.CopyFiles {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}

func CollectRequiredPostScriptFiles(config *types.DevcontainerConfig) ([]string, error) {
	resolved, err := ResolveDockerfileModules(config.Dockerfile.Modules)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var files []string
	for _, r := range resolved {
		if r.Module.PostScriptFiles == nil {
			continue
		}
		for _, f := range r.Module.PostScriptFiles(r.Options) {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}

func CollectRequiredEnvVars(config *types.DevcontainerConfig) []types.RequiredEnvVar {
	selected := types.NormalizeServices(config.Compose.Services)
	var out []types.RequiredEnvVar
	for _, s := range selected {
		svc := catalog.GetComposeService(types.ServiceID(s.ID))
		if svc == nil {
			continue
		}
		out = append(out, svc.RequiresEnv...)
	}
	return out
}

func PlannedComposeNames(config *types.DevcontainerConfig) (containers []string, network string, volumes []string, err error) {
	result := resolveEnabledServices(config)
	enabledIDs := result.enabledIDs
	optionsByID := result.optionsByID

	declaredVolumes := make(map[string]bool)
	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return nil, "", nil, fmt.Errorf("unknown compose service: %s", id)
		}
		for _, v := range svc.Volumes {
			declaredVolumes[v] = true
		}
	}

	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			continue
		}
		opts := optionsByID[id]
		if opts == nil {
			opts = map[string]any{}
		}
		dbUser, dbPass := ResolveDBCredentials()
		rendered := svc.Render(compose.RenderContext{
			ImageName:         config.Image,
			EnabledServiceIDs: enabledIDs,
			Options:           opts,
			DefaultDBUser:     dbUser,
			DefaultDBPassword: dbPass,
		})
		if rendered == nil {
			continue
		}
		base := rendered.ContainerName
		if base == "" {
			base = string(svc.ID)
		}
		containers = append(containers, prefixContainer(config.Workspace, base))
	}

	network = config.Workspace + "-network"
	for v := range declaredVolumes {
		volumes = append(volumes, prefixVolume(config.Workspace, v))
	}
	return containers, network, volumes, nil
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
