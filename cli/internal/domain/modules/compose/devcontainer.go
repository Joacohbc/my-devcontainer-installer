package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

const SSHServiceName = "devcontainer-ssh"

var DevcontainerService = &ServiceSpec{
	ID:     types.ServiceDevcontainer,
	Label:  SSHServiceName + " (main)",
	Always: true,
	Context: func(ctx RenderContext) *types.ContextSection {
		lines := []string{
			"You are in the `" + SSHServiceName + "` container. It runs `sleep infinity` with an",
			"sshd alongside, so there is no application process to keep alive — start and",
			"stop whatever you need.",
		}
		if len(ctx.Ports) > 0 {
			lines = append(lines, "", "Ports published to the host (`host:container`):", "")
			for _, p := range ctx.Ports {
				lines = append(lines, "- `"+p+"`")
			}
			lines = append(lines,
				"",
				"**Only these ports are reachable from the host.** Binding any other port",
				"inside the container does not expose it; the project has to be regenerated",
				"with the port added.")
		} else {
			lines = append(lines,
				"",
				"**No ports are published to the host.** A server you start here is reachable",
				"from sibling containers but not from the host browser, until the project is",
				"regenerated with a port mapping.")
		}
		if ctx.SharedConfigMount != "" {
			lines = append(lines,
				"",
				"The shared tool-config volume is mounted at `"+types.SharedConfigMountPath+"`. The",
				"entrypoint symlinks parts of the home into it (`~/.claude`, `~/.codex`,",
				"`~/.config/gh`, `~/.agents`, `~/"+types.SharedConfigAliasTarget+"`), so tool logins and",
				"agent skills are shared with every other container and survive rebuilds.")
		}
		return &types.ContextSection{Title: "This container", Body: ctxBody(lines...)}
	},
	Render: func(ctx RenderContext) *ServiceDef {
		workspaceDir := ctx.WorkspaceDir
		if workspaceDir == "" {
			workspaceDir = "/workspace"
		}
		volumes := []string{"../..:" + workspaceDir}
		if ctx.SharedConfigMount != "" {
			volumes = append(volumes, ctx.SharedConfigMount)
		}

		var environment any
		if len(ctx.ModuleEnv) > 0 {
			environment = ctx.ModuleEnv
		}

		return &ServiceDef{
			Image:         ctx.ImageName,
			Build:         &BuildDef{Context: "."},
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Environment:   environment,
			Volumes:       volumes,
			Ports:         ctx.Ports,
			Networks:      []string{"local-network"},
		}
	},
}
