package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var NgrokService = &ServiceSpec{
	ID:         types.ServiceNgrok,
	Label:      "ngrok Tunnel",
	UICategory: types.UICategoryDevTools,
	RequiresEnv: []types.RequiredEnvVar{
		{Name: "NGROK_AUTHTOKEN", Prompt: "ngrok Auth Token (NGROK_AUTHTOKEN)"},
	},
	Options: []types.ModuleOption{
		{
			ID:      "port",
			Label:   "Port to expose (inside the devcontainer)",
			Type:    types.ModuleOptionInput,
			Default: "3000",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		port, _ := ctx.Options["port"].(string)
		if port == "" {
			port = "3000"
		}
		return &ServiceDef{
			Image:         "ngrok/ngrok:latest",
			ContainerName: "ngrok_tunnel",
			Restart:       "unless-stopped",
			Command:       fmt.Sprintf("http %s:%s", SSHServiceName, port),
			Environment:   []string{"NGROK_AUTHTOKEN=${NGROK_AUTHTOKEN}"},
			Networks:      []string{"local-network"},
		}
	},
}
