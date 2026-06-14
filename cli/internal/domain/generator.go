package domain

import (
	"fmt"
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
	files, err := CollectRequiredPostScriptFiles(config)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	return fmt.Sprintf(`##
## POST-INSTALL SCRIPTS (available inside the container, not auto-run)
##
COPY %s %s/
RUN chown -R devuser:devuser %s && chmod +x %s/*.sh`,
		strings.Join(files, " "),
		types.PostScriptDir,
		types.PostScriptDir,
		types.PostScriptDir,
	), nil
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
		}

		if svc.ID == types.ServiceDevcontainer && config.Mode == types.BuildModeRemote {
			rendered.Build = ""
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
