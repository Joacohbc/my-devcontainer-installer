package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

const SSHServiceName = "devcontainer-ssh"

var DevcontainerService = &ServiceSpec{
	ID:     types.ServiceDevcontainer,
	Label:  SSHServiceName + " (main)",
	Always: true,
	Render: func(ctx RenderContext) *ServiceDef {
		workspaceDir := ctx.WorkspaceDir
		if workspaceDir == "" {
			workspaceDir = "/workspace"
		}
		volumes := []string{"../..:" + workspaceDir}
		if ctx.SharedConfigMount != "" {
			volumes = append(volumes, ctx.SharedConfigMount)
		}

		return &ServiceDef{
			Image:         ctx.ImageName,
			Build:         &BuildDef{Context: "."},
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Restart:       "unless-stopped",
			Volumes:       volumes,
			Ports:         ctx.Ports,
			Networks:      []string{"local-network"},
		}
	},
}
