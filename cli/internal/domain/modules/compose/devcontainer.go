package compose

import "github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"

const SSHServiceName = "devcontainer-ssh"

var DevcontainerService = &ServiceSpec{
	ID:     types.ServiceDevcontainer,
	Label:  SSHServiceName + " (main)",
	Always: true,
	Render: func(ctx RenderContext) *ServiceDef {
		dbServices := []string{"mongo", "redis", "postgres"}
		depends := []string{}
		for _, id := range ctx.EnabledServiceIDs {
			for _, db := range dbServices {
				if id == db {
					depends = append(depends, id)
				}
			}
		}

		volumes := append([]string{"../..:/workspace"}, ctx.PersistVolumeMounts...)
		if ctx.SharedConfigMount != "" {
			volumes = append(volumes, ctx.SharedConfigMount)
		}

		svc := &ServiceDef{
			Image:         ctx.ImageName,
			Build:         ".",
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Restart:       "unless-stopped",
			Volumes:       volumes,
			Ports:         ctx.Ports,
			Networks:      []string{"local-network"},
		}

		if len(depends) > 0 {
			svc.DependsOn = depends
		}

		return svc
	},
}
