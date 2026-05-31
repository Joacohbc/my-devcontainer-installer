package compose

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var MysqlService = &ServiceSpec{
	ID:         types.ServiceMysql,
	Label:      "MySQL Community Edition",
	UICategory: types.UICategoryDatabases,
	Volumes:    []string{"mysql_data"},
	Options: []types.ModuleOption{
		{
			ID:    "version",
			Label: "MySQL version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "8.0", Label: "MySQL 8.0 (LTS, Recommended)"},
				{Value: "8.4", Label: "MySQL 8.4 (LTS)"},
				{Value: "9.0", Label: "MySQL 9.0 (Innovation)"},
			},
			Default: "8.0",
		},
	},
	Render: func(ctx RenderContext) *ServiceDef {
		version, _ := ctx.Options["version"].(string)
		if version == "" {
			version = "8.0"
		}
		user := ctx.DefaultDBUser
		if user == "" {
			user = "devuser"
		}
		pass := ctx.DefaultDBPassword
		if pass == "" {
			pass = "devpass"
		}
		return &ServiceDef{
			Image:         fmt.Sprintf("mysql:%s", version),
			ContainerName: "mysql",
			Environment: map[string]string{
				"MYSQL_ROOT_PASSWORD": pass,
				"MYSQL_DATABASE":      "devdb",
				"MYSQL_USER":          user,
				"MYSQL_PASSWORD":      pass,
			},
			Volumes:  []string{"mysql_data:/var/lib/mysql"},
			Networks: []string{"local-network"},
		}
	},
}
