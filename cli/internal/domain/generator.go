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
	sort.Strings(enabledIDs)
	return enabledServicesResult{enabledIDs: enabledIDs, optionsByID: optionsByID}
}

type composeContext struct {
	config                *types.DevcontainerConfig
	workspace             string
	networkName           string
	subnet                string
	labels                map[string]string
	enabledIDs            []string
	optionsByID           map[string]map[string]any
	declaredVolumes       map[string]bool
	devcontainerImageName string
	devcontainerIP        string
}

func GenerateCompose(config *types.DevcontainerConfig) (string, error) {
	resolved := resolveEnabledServices(config)

	subnet := config.Compose.Subnet
	if subnet == "" {
		subnet = DefaultSubnet
	}
	devcontainerIP, _ := LastHost(subnet)

	declaredVolumes, err := collectDeclaredVolumes(resolved.enabledIDs)
	if err != nil {
		return "", err
	}

	ctx := composeContext{
		config:                config,
		workspace:             config.Workspace,
		networkName:           config.Workspace + "-network",
		subnet:                subnet,
		labels:                types.ComposeLabels(config),
		enabledIDs:            resolved.enabledIDs,
		optionsByID:           resolved.optionsByID,
		declaredVolumes:       declaredVolumes,
		devcontainerImageName: ResolveDevcontainerImageName(config),
		devcontainerIP:        devcontainerIP,
	}

	services, err := ctx.renderServices()
	if err != nil {
		return "", err
	}

	doc := compose.ComposeDoc{
		Name:     ctx.workspace,
		Services: services,
		Networks: ctx.networks(),
		Volumes:  ctx.volumes(),
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return types.GeneratedHeader + "\n" + string(out), nil
}

func collectDeclaredVolumes(enabledIDs []string) (map[string]bool, error) {
	declared := make(map[string]bool)
	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return nil, fmt.Errorf("unknown compose service: %s", id)
		}
		for _, v := range svc.Volumes {
			declared[v] = true
		}
	}
	return declared, nil
}

func (c composeContext) renderServices() (map[string]*compose.ServiceDef, error) {
	services := make(map[string]*compose.ServiceDef)
	for _, id := range c.enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return nil, fmt.Errorf("unknown compose service: %s", id)
		}
		key, rendered := c.renderService(svc)
		if rendered == nil {
			continue
		}
		services[key] = rendered
	}
	return services, nil
}

func (c composeContext) renderService(svc *compose.ServiceSpec) (string, *compose.ServiceDef) {
	isDevcontainer := svc.ID == types.ServiceDevcontainer

	rendered := svc.Render(c.renderContext(svc, isDevcontainer))
	if rendered == nil {
		return "", nil
	}
	if isDevcontainer {
		c.configureDevcontainer(rendered)
	}

	baseContainer := rendered.ContainerName
	if baseContainer == "" {
		baseContainer = string(svc.ID)
	}
	rendered.ContainerName = prefixContainer(c.workspace, baseContainer)
	rendered.Volumes = c.remapVolumes(rendered.Volumes)
	rendered.Networks = remapServiceNetworks(rendered.Networks, c.networkName, isDevcontainer, c.devcontainerIP)
	rendered.Labels = copyLabels(c.labels)

	if isDevcontainer {
		return compose.SSHServiceName, rendered
	}
	return string(svc.ID), rendered
}

func (c composeContext) renderContext(svc *compose.ServiceSpec, isDevcontainer bool) compose.RenderContext {
	imageName := c.config.Image
	if isDevcontainer {
		imageName = c.devcontainerImageName
	}
	options := c.optionsByID[string(svc.ID)]
	if options == nil {
		options = map[string]any{}
	}
	dbUser, dbPass := ResolveDBCredentials()
	rc := compose.RenderContext{
		ImageName:         imageName,
		EnabledServiceIDs: c.enabledIDs,
		Options:           options,
		DefaultDBUser:     dbUser,
		DefaultDBPassword: dbPass,
	}
	if isDevcontainer {
		rc.Ports = devcontainerPorts(c.config)
		rc.WorkspaceDir = types.WorkspaceDir(c.workspace)
		if types.SharedConfigEnabled(c.config) {
			rc.SharedConfigMount = types.SharedConfigMount()
		}
	}
	return rc
}

func (c composeContext) configureDevcontainer(rendered *compose.ServiceDef) {
	rendered.DependsOn = c.databaseDependencies()

	if c.dockerOutsideDockerEnabled() {
		rendered.Volumes = append(rendered.Volumes, "/var/run/docker.sock:/var/run/docker.sock")
	}
	// User-defined extra mounts are appended verbatim; named-volume sources are
	// declared in the top-level volumes section (see volumes()).
	rendered.Volumes = append(rendered.Volumes, c.config.Compose.Volumes...)

	if c.config.Mode == types.BuildModeRemote {
		rendered.Build = nil
	} else {
		rendered.Build = devcontainerBuild(c.config)
	}
	rendered.Hostname = c.workspace
}

// databaseDependencies returns the enabled compose services flagged as
// databases in the catalog (already sorted, since enabledIDs is sorted), for the
// devcontainer's depends_on. Returns nil when there are none.
func (c composeContext) databaseDependencies() []string {
	var deps []string
	for _, id := range c.enabledIDs {
		if svc := catalog.GetComposeService(types.ServiceID(id)); svc != nil && svc.IsDatabase {
			deps = append(deps, id)
		}
	}
	return deps
}

func (c composeContext) dockerOutsideDockerEnabled() bool {
	for _, m := range c.config.Dockerfile.Modules {
		if m.ID == types.ModuleDod {
			return true
		}
	}
	return false
}

func (c composeContext) remapVolumes(mounts []string) []string {
	remapped := make([]string, len(mounts))
	for i, mount := range mounts {
		remapped[i] = remapVolumeMount(c.workspace, c.declaredVolumes, mount)
	}
	return remapped
}

func (c composeContext) volumes() map[string]*compose.VolumeDef {
	volumes := make(map[string]*compose.VolumeDef)
	for v := range c.declaredVolumes {
		volumes[prefixVolume(c.workspace, v)] = &compose.VolumeDef{Labels: copyLabels(c.labels)}
	}
	// Declare named-volume sources from the user's extra mounts so compose does
	// not reject them as undefined. Bind-mount sources (host paths) need no
	// declaration. Names are kept verbatim (compose prefixes them by project).
	for _, mount := range c.config.Compose.Volumes {
		name, ok := userNamedVolume(mount)
		if !ok {
			continue
		}
		if _, exists := volumes[name]; !exists {
			volumes[name] = &compose.VolumeDef{Labels: copyLabels(c.labels)}
		}
	}
	// The shared tool-config volume is daemon-level (shared by every workspace),
	// so it is declared external (never prefixed, never removed by compose down -v).
	if types.SharedConfigEnabled(c.config) {
		volumes[types.SharedConfigVolumeName] = &compose.VolumeDef{External: true}
	}
	return volumes
}

func (c composeContext) networks() map[string]*compose.NetworkDef {
	return map[string]*compose.NetworkDef{
		c.networkName: {
			Driver: "bridge",
			IPAM: &compose.IPAMConfig{
				Config: []compose.IPAMSubnet{
					{Subnet: fmt.Sprintf("${DOCKER_SUBNET:-%s}", c.subnet)},
				},
			},
			Labels: copyLabels(c.labels),
		},
	}
}

func remapServiceNetworks(raw any, networkName string, isDevcontainer bool, devcontainerIP string) any {
	networkNames, ok := raw.([]string)
	if !ok {
		return raw
	}
	mapped := make([]string, len(networkNames))
	for i, name := range networkNames {
		mapped[i] = mapLocalNetwork(name, networkName)
	}
	if !isDevcontainer || devcontainerIP == "" {
		return mapped
	}
	withAddress := make(map[string]any, len(mapped))
	for _, name := range mapped {
		if name == networkName {
			withAddress[name] = map[string]any{
				"ipv4_address": fmt.Sprintf("${DEVCONTAINER_IP:-%s}", devcontainerIP),
			}
			continue
		}
		withAddress[name] = nil
	}
	return withAddress
}

func mapLocalNetwork(name, networkName string) string {
	if name == "local-network" {
		return networkName
	}
	return name
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

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := []string{"# Generated by devcontainer CLI"}
	for _, k := range keys {
		lines = append(lines, k+"="+env[k])
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
	sort.Strings(volumes)
	return containers, network, volumes, nil
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
