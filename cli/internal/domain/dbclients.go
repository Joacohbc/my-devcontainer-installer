package domain

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// dbClientLink ties a DB client Dockerfile module to the compose service whose
// version it mirrors when the client's "version" option is left on auto.
type dbClientLink struct {
	client  types.ModuleID
	service types.ServiceID
	// translate converts the service's version option into the client's version
	// option value (e.g. "17-alpine" -> "17"). Returns "" when it cannot map it.
	translate func(serviceVersion string) string
}

var dbClientLinks = []dbClientLink{
	{client: types.ModulePostgresClient, service: types.ServicePostgres, translate: majorBeforeDash},
}

// majorBeforeDash returns the leading segment before the first '-', e.g.
// "17-alpine" -> "17". Postgres/redis service versions carry an image suffix
// the client package version must not include.
func majorBeforeDash(v string) string {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i]
	}
	return v
}

// MatchDBClientVersions returns the selected Dockerfile modules with each DB
// client's "version" option resolved: when it is unset or "auto" and the
// matching DB service is selected, it is set to mirror that service's version.
// A client left on auto with no matching service keeps auto (generic install).
// The input config is not mutated.
func MatchDBClientVersions(config *types.DevcontainerConfig) []types.SelectedModule {
	serviceVersions := selectedServiceVersions(config)

	out := make([]types.SelectedModule, len(config.Dockerfile.Modules))
	for i, m := range config.Dockerfile.Modules {
		out[i] = cloneSelectedModule(m)
	}

	for _, link := range dbClientLinks {
		serviceVersion, ok := serviceVersions[link.service]
		if !ok {
			continue
		}
		matched := link.translate(serviceVersion)
		if matched == "" {
			continue
		}
		for i := range out {
			if out[i].ID != link.client {
				continue
			}
			if cur, _ := out[i].Options["version"].(string); cur == "" || cur == "auto" {
				out[i].Options["version"] = matched
			}
		}
	}
	return out
}

// selectedServiceVersions maps each selected compose service to its effective
// version: the explicit option when set, otherwise the service's catalog
// default (which is what the service would render anyway).
func selectedServiceVersions(config *types.DevcontainerConfig) map[types.ServiceID]string {
	res := make(map[types.ServiceID]string)
	for _, s := range config.Compose.Services {
		version := types.StringOpt(s.Options, "version", defaultServiceVersion(s.ID))
		res[s.ID] = version
	}
	return res
}

// defaultServiceVersion returns the catalog default for a service's "version"
// option, or "" if it has none.
func defaultServiceVersion(id types.ServiceID) string {
	svc := catalog.GetComposeService(id)
	if svc == nil {
		return ""
	}
	for _, o := range svc.Options {
		if o.ID == "version" {
			if d, ok := o.Default.(string); ok {
				return d
			}
		}
	}
	return ""
}

func cloneSelectedModule(m types.SelectedModule) types.SelectedModule {
	opts := make(map[string]any, len(m.Options))
	for k, v := range m.Options {
		opts[k] = v
	}
	return types.SelectedModule{ID: m.ID, Options: opts}
}
