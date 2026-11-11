package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

const DefaultSubnet = "172.25.0.0/28"

func GenerateDockerfile(config *types.DevcontainerConfig) (string, error) {
	if config.Mode == types.BuildModeProfiles || config.Mode == types.BuildModeImage {
		return "", nil
	}
	matched := MatchDBClientVersions(config)
	resolved, err := ResolveDockerfileModules(append(append([]types.SelectedModule{}, config.Dockerfile.BaseModules...), matched...))
	if err != nil {
		return "", err
	}
	installed, err := baseImageModules(config)
	if err != nil {
		return "", err
	}
	fragments := make([]string, 0, len(resolved)+5)
	if config.Dockerfile.From != "" {
		fragments = append(fragments, derivedFromBlock(config.Dockerfile.BaseRef))
	}
	for _, r := range resolved {
		if installed[r.Module.ID] {
			continue
		}
		fragments = append(fragments, strings.TrimSpace(r.Module.Render(r.Options)))
	}
	// The environment is rendered from every module, inherited ones included:
	// the derived ENV block and ~/.dc-env.sh replace the parent's.
	fragments = append(fragments, renderEnvironmentBlocks(resolved, config.Dockerfile.From != "")...)
	postScripts, err := postScriptsDockerfileBlock(config)
	if err != nil {
		return "", err
	}
	if postScripts != "" {
		fragments = append(fragments, postScripts)
	}
	customScripts, err := customScriptsDockerfileBlock(config)
	if err != nil {
		return "", err
	}
	if customScripts != "" {
		fragments = append(fragments, customScripts)
	}
	fragments = append(fragments, types.DockerfileLabelBlock(config))
	return types.GeneratedHeader + "\n\n" + strings.Join(fragments, "\n\n") + "\n", nil
}

// baseImageModules is what a derived build's parent image already installed:
// its resolved modules (always-on ones included), minus cleanup, which every
// build re-renders so the entrypoint and apt state match this CLI version.
// It is empty for a full build.
func baseImageModules(config *types.DevcontainerConfig) (map[types.ModuleID]bool, error) {
	installed := map[types.ModuleID]bool{}
	if config.Dockerfile.From == "" {
		return installed, nil
	}
	if config.Dockerfile.BaseRef == "" {
		return nil, fmt.Errorf("base image %q was not resolved before generating the Dockerfile", config.Dockerfile.From)
	}
	base, err := ResolveDockerfileModules(config.Dockerfile.BaseModules)
	if err != nil {
		return nil, fmt.Errorf("base image %s: %w", config.Dockerfile.From, err)
	}
	for _, r := range base {
		if r.Module.ID != types.ModuleCleanup {
			installed[r.Module.ID] = true
		}
	}
	// A module the base already has is not reinstalled, so asking for it with
	// other options would build one thing and describe another.
	baseOptions := map[types.ModuleID]map[string]any{}
	for _, m := range config.Dockerfile.BaseModules {
		baseOptions[m.ID] = m.Options
	}
	for _, m := range config.Dockerfile.Modules {
		if !installed[m.ID] || len(m.Options) == 0 {
			continue
		}
		if !sameOptions(m.Options, baseOptions[m.ID]) {
			return nil, fmt.Errorf("module %s is already installed in base image %s with other options; drop it from the modules or change the base image", m.ID, config.Dockerfile.From)
		}
	}
	return installed, nil
}

// derivedFromBlock starts a derived build. The parent was built with its own
// USER_UID/USER_GID (1000 for a named image); when the host ids differ,
// devuser is remapped so it still owns the bind-mounted workspace. usermod
// re-owns the home directory itself; every other path the parent gave devuser
// (the Python sysconfig dirs, /opt/android-sdk, ...) is found by its old ids.
func sameOptions(a, b map[string]any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

func derivedFromBlock(ref string) string {
	return fmt.Sprintf(`##
## BASE IMAGE (derived build)
##
FROM %s
USER root
ARG USER_UID=1000
ARG USER_GID=1000
RUN old_uid="$(id -u devuser)" && old_gid="$(id -g devuser)" && \
    if [ "${old_uid}" != "${USER_UID}" ] || [ "${old_gid}" != "${USER_GID}" ]; then \
        groupmod -o -g "${USER_GID}" devuser && \
        usermod -o -u "${USER_UID}" -g "${USER_GID}" devuser && \
        find / -xdev -path /proc -prune -o -user "${old_uid}" -exec chown -h "${USER_UID}" {} + && \
        find / -xdev -path /proc -prune -o -group "${old_gid}" -exec chgrp -h "${USER_GID}" {} +; \
    fi`, ref)
}

// renderEnvironmentBlocks turns every resolved module's declared environment
// into the Dockerfile fragments that deliver it. Both blocks come from the same
// merged value: one ENV section a process inherits without any shell, and one
// sourced script for shells that start before it.
//
// They are emitted after every module, never inside one: the leading layers of
// the Dockerfile must stay byte-identical across image variants for the shared
// base cache to be reused, and an ENV instruction whose content depends on the
// selected modules would break that if it came earlier.
func renderEnvironmentBlocks(resolved []ResolvedModule, derived bool) []string {
	envs := make([]dockerfile.ContainerEnv, 0, len(resolved))
	for _, r := range resolved {
		if r.Module.ProvidesEnv == nil {
			continue
		}
		envs = append(envs, r.Module.ProvidesEnv(r.Options))
	}
	merged := dockerfile.MergeContainerEnv(envs...)
	var blocks []string
	envBlock := dockerfile.RenderDockerfileEnv(merged)
	if derived {
		envBlock = dockerfile.RenderDerivedDockerfileEnv(merged)
	}
	for _, block := range []string{envBlock, dockerfile.RenderEnvScript(merged)} {
		if block != "" {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func postScriptsDockerfileBlock(config *types.DevcontainerConfig) (string, error) {
	manual, autoStart, services, err := collectPartitionedPostScripts(config)
	if err != nil {
		return "", err
	}
	if len(manual) == 0 && len(autoStart) == 0 && len(services) == 0 {
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
		// runs agents first and downstream scripts/skills last.
		for _, e := range autoStart {
			lines = append(lines, fmt.Sprintf("COPY %s %s/%02d-%s", e.File, types.PostScriptStartDir, e.Order, e.File))
			// Also expose each auto-start script under its plain name in
			// ~/post-script/ (a relative symlink into start.d/) so the user can
			// still run it manually, exactly like the interactive scripts.
			symlinks = append(symlinks, fmt.Sprintf("ln -sfn start.d/%02d-%s %s/%s", e.Order, e.File, types.PostScriptDir, e.File))
		}
		chmodTargets = append(chmodTargets, types.PostScriptStartDir+"/*.sh")
	}
	if len(services) > 0 {
		// Background services the entrypoint runs on every container start
		// (without a .done sentinel).
		lines = append(lines, fmt.Sprintf("COPY %s %s/", strings.Join(services, " "), types.PostScriptServicesDir))
		for _, f := range services {
			symlinks = append(symlinks, fmt.Sprintf("ln -sfn services.d/%s %s/%s", f, types.PostScriptDir, f))
		}
		chmodTargets = append(chmodTargets, types.PostScriptServicesDir+"/*.sh")
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

// customScriptsDockerfileBlock bakes the user's own `when: build` scripts into
// the image. It is emitted last, after every module and after the environment
// blocks, for two reasons: the leading layers must stay byte-identical across
// image variants so the devcontainer-base cache is reused, and a user script is
// meant to build on the toolchain the modules just installed.
//
// The scripts run as devuser (passwordless sudo is available inside them, like
// everywhere else in this image) and the staging directory is removed in the
// same layer, so nothing of it survives into the final image.
func customScriptsDockerfileBlock(config *types.DevcontainerConfig) (string, error) {
	build, _, _, _, err := PartitionCustomScripts(config)
	if err != nil {
		return "", err
	}
	if len(build) == 0 {
		return "", nil
	}

	const stageDir = "/tmp/devcontainer-custom-scripts"
	lines := []string{"##", "## CUSTOM SCRIPTS (user-provided, run at build time)", "##"}
	lines = append(lines, fmt.Sprintf("COPY %s %s/", strings.Join(build, " "), stageDir))

	run := fmt.Sprintf("RUN chown -R devuser:devuser %s && chmod +x %s/*.sh", stageDir, stageDir)
	for _, f := range build {
		// The file name is validated to a shell-safe character set, so the
		// single-quoted word below cannot be broken out of.
		run += fmt.Sprintf(" \\\n    && su - devuser -c '%s/%s'", stageDir, f)
	}
	run += fmt.Sprintf(" \\\n    && rm -rf %s", stageDir)
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
func collectPartitionedPostScripts(config *types.DevcontainerConfig) (manual []string, autoStart []postScriptStart, services []string, err error) {
	resolved, err := ResolveDockerfileModules(config.Dockerfile.Modules)
	if err != nil {
		return nil, nil, nil, err
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
	// The user's own scripts join three buckets: `when: start` is an
	// auto-start installer (ordered after every module-provided one, so it can
	// build on them), `when: service` is a background daemon started on every boot,
	// and `when: manual` is left under PostScriptDir to be run by hand.
	_, customStart, customService, customManual, err := PartitionCustomScripts(config)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, f := range customStart {
		if seen[f] {
			continue
		}
		seen[f] = true
		autoStart = append(autoStart, postScriptStart{File: f, Order: types.CustomScriptStartOrder})
	}
	appendUnseen := func(dst *[]string, src []string) {
		for _, f := range src {
			if !seen[f] {
				seen[f] = true
				*dst = append(*dst, f)
			}
		}
	}
	appendUnseen(&services, customService)
	appendUnseen(&manual, customManual)

	sort.Slice(autoStart, func(i, j int) bool {
		if autoStart[i].Order != autoStart[j].Order {
			return autoStart[i].Order < autoStart[j].Order
		}
		return autoStart[i].File < autoStart[j].File
	})
	return manual, autoStart, services, nil
}

func ResolveRemoteImage(variant, registry string) string {
	if registry == "" {
		registry = DefaultRegistry
	}
	return registry + "devcontainer-" + variant + ":latest"
}

func ResolveDevcontainerImageName(config *types.DevcontainerConfig) string {
	if config.Mode == types.BuildModeProfiles && config.Remote != nil {
		reg := config.Remote.Registry
		return ResolveRemoteImage(config.Remote.Variant, reg)
	}
	if config.Mode == types.BuildModeImage {
		return config.Image
	}
	if config.Mode == types.BuildModeCustom && config.Fingerprint != "" && config.ImageKind != types.ImageKindNamed {
		return ContentImageTag(config.ResolvedOrigin(), config.Fingerprint)
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
	selected := config.Compose.Services
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
		networkName:           WorkspaceNetworkName(config.Workspace),
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

// unknownServiceError reports a service id with no catalog entry. The removed
// "tunnel" service gets a pointer at its replacement: LoadConfig migrates stored
// configs, but "--service tunnel" typed from an old script or README reaches
// here untouched.
func unknownServiceError(id string) error {
	if id == legacyTunnelService {
		return fmt.Errorf("compose service %q was replaced by the %q dockerfile module, which installs cloudflared inside the container: use --modules %s", id, types.ModuleCloudflared, types.ModuleCloudflared)
	}
	return fmt.Errorf("unknown compose service: %s", id)
}

func collectDeclaredVolumes(enabledIDs []string) (map[string]bool, error) {
	declared := make(map[string]bool)
	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return nil, unknownServiceError(id)
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
			return nil, unknownServiceError(id)
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
		Options:           options,
		DefaultDBUser:     dbUser,
		DefaultDBPassword: dbPass,
	}
	if isDevcontainer {
		rc.Ports = devcontainerPorts(c.config)
		rc.ModuleEnv = append(devcontainerModuleEnv(c.config), SkillsEnv(c.config.Skills)...)
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
	rendered.Volumes = append(rendered.Volumes, ContextMount())

	if c.config.Mode == types.BuildModeProfiles || c.config.Mode == types.BuildModeImage {
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
	return hasModule(c.config.Dockerfile.Modules, types.ModuleDod)
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

// CollectRequiredEnvVars returns the env vars the selected compose services and
// Dockerfile modules need, services first and without duplicates. It drives the
// generate wizard's prompt steps.
func CollectRequiredEnvVars(config *types.DevcontainerConfig) []types.RequiredEnvVar {
	return dedupeEnvVars(append(serviceEnvVars(config), moduleEnvVars(config)...))
}

func serviceEnvVars(config *types.DevcontainerConfig) []types.RequiredEnvVar {
	var vars []types.RequiredEnvVar
	for _, s := range config.Compose.Services {
		if svc := catalog.GetComposeService(s.ID); svc != nil {
			vars = append(vars, svc.RequiresEnv...)
		}
	}
	return vars
}

func moduleEnvVars(config *types.DevcontainerConfig) []types.RequiredEnvVar {
	var vars []types.RequiredEnvVar
	for _, m := range config.Dockerfile.Modules {
		if mod := catalog.GetDockerfileModule(m.ID); mod != nil {
			vars = append(vars, mod.RequiresEnv...)
		}
	}
	return vars
}

func dedupeEnvVars(vars []types.RequiredEnvVar) []types.RequiredEnvVar {
	seen := make(map[string]bool, len(vars))
	deduped := make([]types.RequiredEnvVar, 0, len(vars))
	for _, v := range vars {
		if seen[v.Name] {
			continue
		}
		seen[v.Name] = true
		deduped = append(deduped, v)
	}
	return deduped
}

// devcontainerModuleEnv renders the compose `environment:` entries that carry
// the selected modules' env vars into the container. The "${NAME:-}" default is
// what makes an unanswered var a supported state instead of a compose warning:
// the value is simply absent from the .env and arrives empty.
//
// The project's own env (devcontainer.config.json `env`, which a profile's
// `env` fills) is forwarded the same way, so a variable defined there reaches
// the container and not only compose interpolation.
func devcontainerModuleEnv(config *types.DevcontainerConfig) []string {
	vars := dedupeEnvVars(moduleEnvVars(config))
	seen := make(map[string]bool, len(vars)+len(config.Env))
	env := make([]string, 0, len(vars)+len(config.Env))
	for _, v := range vars {
		seen[v.Name] = true
		env = append(env, fmt.Sprintf("%s=${%s:-}", v.Name, v.Name))
	}
	names := make([]string, 0, len(config.Env))
	for name := range config.Env {
		if !seen[name] && !composeOnlyEnv[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		env = append(env, fmt.Sprintf("%s=${%s:-}", name, name))
	}
	return env
}

// composeOnlyEnv are .env entries that only steer compose itself.
var composeOnlyEnv = map[string]bool{"COMPOSE_PROJECT_NAME": true, "DEVCONTAINER_IP": true, "DOCKER_SUBNET": true}

func PlannedComposeNames(config *types.DevcontainerConfig) (containers []string, network string, volumes []string, err error) {
	result := resolveEnabledServices(config)
	enabledIDs := result.enabledIDs
	optionsByID := result.optionsByID

	declaredVolumes := make(map[string]bool)
	for _, id := range enabledIDs {
		svc := catalog.GetComposeService(types.ServiceID(id))
		if svc == nil {
			return nil, "", nil, unknownServiceError(id)
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

	network = WorkspaceNetworkName(config.Workspace)
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

// ContextMount bind-mounts the project's generated CONTEXT.md read-only into
// the devcontainer. It is mounted rather than baked because it names the
// workspace path: baking it made every workspace's image unique, so identical
// content could never be shared. Mounting also gives prebuilt (profiles/image)
// containers an accurate context, which a baked file never could.
func ContextMount() string {
	return "./" + types.ContextFileName + ":/home/devuser/" + types.ContextFileName + ":ro"
}
