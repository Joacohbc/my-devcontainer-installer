package compose

const SSHServiceName = "devcontainer-ssh"

var DevcontainerService = &ServiceSpec{
	ID:     "devcontainer",
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

		svc := &ServiceDef{
			Image:         ctx.ImageName,
			Build:         ".",
			ContainerName: SSHServiceName,
			Command:       "sleep infinity",
			Restart:       "unless-stopped",
			Volumes: []string{
				"../..:/workspace",
				"devcontainer_etc:/etc",
				"devcontainer_root:/root",
				"devcontainer_home:/home",
			},
			Networks: []string{"local-network"},
		}

		if len(depends) > 0 {
			svc.DependsOn = depends
		}

		return svc
	},
}
