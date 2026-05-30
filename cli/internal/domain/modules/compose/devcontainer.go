package compose

const SSHServiceName = "devcontainer-ssh"

var DevcontainerService = &ServiceSpec{
	ID:     "devcontainer",
	Label:  SSHServiceName + " (main)",
	Always: true,
	Render: func(ctx RenderContext) *ServiceDef {
		dbServices := []string{"mongo", "redis", "postgres", "mysql"}
		depends := []string{}
		for _, id := range ctx.EnabledServiceIDs {
			for _, db := range dbServices {
				if id == db {
					depends = append(depends, id)
				}
			}
		}

		volumes := append([]string{"../..:/workspace"}, ctx.PersistVolumeMounts...)

		svc := &ServiceDef{
			Image:         ctx.ImageName,
			Build:         ".",
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Restart:       "unless-stopped",
			Volumes:       volumes,
			Networks:      []string{"local-network"},
		}

		if len(depends) > 0 {
			svc.DependsOn = depends
		}

		return svc
	},
}
