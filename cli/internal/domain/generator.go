package domain

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/core"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/registry"
)

const DefaultSubnet = "172.25.0.0/28"

func GenerateDockerfile(config *core.DevcontainerConfig) (string, error) {
	if config.Mode == core.BuildModeRemote {
		return "", nil
	}
	resolved, err := ResolveDockerfileModules(config.Dockerfile.Modules)
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
	fragments = append(fragments, core.DockerfileLabelBlock(config))
	return core.GeneratedHeader + "\n\n" + strings.Join(fragments, "\n\n") + "\n", nil
}

func postScriptsDockerfileBlock(config *core.DevcontainerConfig) (string, error) {
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
		core.PostScriptDir,
		core.PostScriptDir,
		core.PostScriptDir,
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

func ResolveDevcontainerImageName(config *core.DevcontainerConfig) string {
	if config.Mode == core.BuildModeRemote && config.Remote != nil {
		reg := config.Remote.Registry
		return ResolveRemoteImage(config.Remote.Variant, reg)
	}
	if config.Mode == core.BuildModeLocalCached && config.Fingerprint != "" {
		fp := config.Fingerprint
		if len(fp) > 12 {
			fp = fp[:12]
		}
		return core.ImageNamespace + "/" + fp + ":latest"
	}
	return config.Image
}

var baseVolumes = []string{"devcontainer_etc", "devcontainer_root", "devcontainer_home"}

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

func resolveEnabledServices(config *core.DevcontainerConfig) enabledServicesResult {
	selected := core.NormalizeServices(config.Compose.Services)
	enabled := make(map[string]bool)
	optionsByID := make(map[string]map[string]any)

	for _, s := range selected {
		enabled[s.ID] = true
		opts := s.Options
		if opts == nil {
			opts = map[string]any{}
		}
		optionsByID[s.ID] = opts
	}

	for _, svc := range registry.ComposeServices {
		if svc.Always {
			enabled[svc.ID] = true
		}
	}

	enabledIDs := make([]string, 0, len(enabled))
	for id := range enabled {
		enabledIDs = append(enabledIDs, id)
	}
	return enabledServicesResult{enabledIDs: enabledIDs, optionsByID: optionsByID}
}

func GenerateCompose(config *core.DevcontainerConfig) (string, error) {
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

	labels := core.ComposeLabels(config)
	subnet := config.Compose.Subnet
	if subnet == "" {
		subnet = DefaultSubnet
	}
	devcontainerIP, _ := LastHost(subnet)

	declaredVolumes := make(map[string]bool)
	for _, v := range baseVolumes {
		declaredVolumes[v] = true
	}
	for _, id := range enabledIDs {
		svc := registry.GetComposeService(id)
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
		svc := registry.GetComposeService(id)
		if svc == nil {
			return "", fmt.Errorf("unknown compose service: %s", id)
		}

		imageNameForSvc := config.Image
		if svc.ID == "devcontainer" {
			imageNameForSvc = devcontainerImageName
		}

		opts := optionsByID[id]
		if opts == nil {
			opts = map[string]any{}
		}

		rendered := svc.Render(compose.RenderContext{
			ImageName:         imageNameForSvc,
			EnabledServiceIDs: enabledIDs,
			Options:           opts,
		})
		if rendered == nil {
			continue
		}

		if svc.ID == "devcontainer" && config.Mode == core.BuildModeRemote {
			rendered.Build = ""
		}

		baseContainer := rendered.ContainerName
		if baseContainer == "" {
			baseContainer = svc.ID
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
			if svc.ID == "devcontainer" && devcontainerIP != "" {
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

		serviceKey := svc.ID
		if svc.ID == "devcontainer" {
			serviceKey = compose.SSHServiceName
		}
		services[serviceKey] = rendered
	}

	volumes := make(map[string]*compose.VolumeDef)
	for v := range declaredVolumes {
		volumes[prefixVolume(workspace, v)] = &compose.VolumeDef{Labels: copyLabels(labels)}
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
		Services: services,
		Networks: networks,
		Volumes:  volumes,
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return core.GeneratedHeader + "\n" + string(out), nil
}

func GenerateEnv(config *core.DevcontainerConfig) string {
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
	lines := []string{"# Generated by devcontainer CLI"}
	for k, v := range env {
		lines = append(lines, k+"="+v)
	}
	return strings.Join(lines, "\n") + "\n"
}

func CollectRequiredCopyFiles(config *core.DevcontainerConfig) ([]string, error) {
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

func CollectRequiredPostScriptFiles(config *core.DevcontainerConfig) ([]string, error) {
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

func CollectRequiredEnvVars(config *core.DevcontainerConfig) []core.RequiredEnvVar {
	selected := core.NormalizeServices(config.Compose.Services)
	var out []core.RequiredEnvVar
	for _, s := range selected {
		svc := registry.GetComposeService(s.ID)
		if svc == nil {
			continue
		}
		out = append(out, svc.RequiresEnv...)
	}
	return out
}

func PlannedComposeNames(config *core.DevcontainerConfig) (containers []string, network string, volumes []string, err error) {
	result := resolveEnabledServices(config)
	enabledIDs := result.enabledIDs
	optionsByID := result.optionsByID

	declaredVolumes := make(map[string]bool)
	for _, v := range baseVolumes {
		declaredVolumes[v] = true
	}
	for _, id := range enabledIDs {
		svc := registry.GetComposeService(id)
		if svc == nil {
			return nil, "", nil, fmt.Errorf("unknown compose service: %s", id)
		}
		for _, v := range svc.Volumes {
			declaredVolumes[v] = true
		}
	}

	for _, id := range enabledIDs {
		svc := registry.GetComposeService(id)
		if svc == nil {
			continue
		}
		opts := optionsByID[id]
		if opts == nil {
			opts = map[string]any{}
		}
		rendered := svc.Render(compose.RenderContext{
			ImageName:         config.Image,
			EnabledServiceIDs: enabledIDs,
			Options:           opts,
		})
		if rendered == nil {
			continue
		}
		base := rendered.ContainerName
		if base == "" {
			base = svc.ID
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
