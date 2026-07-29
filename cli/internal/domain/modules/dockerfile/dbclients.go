package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// aptClientModule builds a ModuleSpec for a database client installed from apt
// with a fixed package (used by the version-agnostic clients: redis, mongo).
// Each client is its own selectable module under the "clients" UI category so
// they can be picked individually in the wizard. setupCmds are optional shell
// commands (e.g. adding a vendor apt repo) chained into the same RUN, before the
// install, so the whole client is a single layer. section is the module's entry
// in the generated ~/CONTEXT.md; these clients take no options, so it is a fixed
// value rather than a function of opts.
func aptClientModule(id types.ModuleID, label, title string, setupCmds []string, pkg string, section *types.ContextSection) *ModuleSpec {
	return &ModuleSpec{
		ID:         id,
		Label:      label,
		Category:   types.CategoryDB,
		UICategory: types.UICategoryClients,
		Context: func(opts map[string]any) *types.ContextSection {
			return section
		},
		Render: func(opts map[string]any) string {
			setup := ""
			for _, c := range setupCmds {
				setup += c + " && \\\n    "
			}
			return fmt.Sprintf(`##
## %s
##
RUN %sapt-get update && apt-get install -y \
    %s && %s
`, title, setup, pkg, aptCleanup())
		},
	}
}

// versionOption builds the shared "version" select for a versioned DB client.
// "auto" mirrors the selected DB service version (resolved by the domain layer);
// any other value pins that release via the vendor apt repo.
func versionOption(label string, choices []types.ModuleOptionChoice) types.ModuleOption {
	all := append([]types.ModuleOptionChoice{
		{Value: "auto", Label: "auto (match the selected DB service)"},
	}, choices...)
	return types.ModuleOption{
		ID:      "version",
		Label:   label,
		Type:    types.ModuleOptionSelect,
		Choices: all,
		Default: "auto",
	}
}

// PostgresClientVersions are the psql releases installable from the PGDG repo.
var PostgresClientVersions = []string{"16", "17", "18"}

// isPostgresClientVersion reports whether v names a pinned PGDG release.
func isPostgresClientVersion(v string) bool {
	for _, pv := range PostgresClientVersions {
		if v == pv {
			return true
		}
	}
	return false
}

var PostgresClientModule = &ModuleSpec{
	ID:         types.ModulePostgresClient,
	Label:      "PostgreSQL client (psql)",
	Category:   types.CategoryDB,
	UICategory: types.UICategoryClients,
	Options: []types.ModuleOption{
		versionOption("psql version", []types.ModuleOptionChoice{
			{Value: "18", Label: "PostgreSQL 18 (PGDG)"},
			{Value: "17", Label: "PostgreSQL 17 (PGDG)"},
			{Value: "16", Label: "PostgreSQL 16 (PGDG)"},
		}),
	},
	Context: func(opts map[string]any) *types.ContextSection {
		version := types.StringOpt(opts, "version", "")
		which := "the version from the Ubuntu repositories"
		if isPostgresClientVersion(version) {
			which = "PostgreSQL " + version + " from the PGDG repository"
		}
		return &types.ContextSection{
			Title: "PostgreSQL client",
			Body: ctxBody(
				"`psql` is installed ("+which+"). It is only the client — the",
				"server, if any, is a sibling container described below.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		version := types.StringOpt(opts, "version", "")
		if !isPostgresClientVersion(version) {
			// Generic client from Ubuntu's repos (no version pin requested).
			return fmt.Sprintf(`##
## POSTGRESQL CLIENT
##
RUN apt-get update && apt-get install -y \
    postgresql-client && %s
`, aptCleanup())
		}
		// Pin the exact major via the official PGDG apt repository.
		return fmt.Sprintf(`##
## POSTGRESQL CLIENT (PGDG %s)
##
RUN install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /etc/apt/keyrings/postgresql.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/postgresql.gpg] https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    apt-get update && \
    apt-get install -y postgresql-client-%s && \
    %s
`, version, version, aptCleanup())
	},
}

var RedisClientModule = aptClientModule(
	types.ModuleRedisClient,
	"Redis client (redis-cli)",
	"REDIS CLIENT",
	nil,
	"redis-tools",
	&types.ContextSection{
		Title: "Redis client",
		Body: ctxBody(
			"`redis-cli` is installed. It is only the client — the server, if any, is a",
			"sibling container described below.",
		),
	},
)

// mongoClientSetup adds the MongoDB apt repo (mongosh is decoupled from the
// server version, so a single 8.0 repo serves every selectable server release).
var mongoClientSetup = []string{
	"install -m 0755 -d /etc/apt/keyrings",
	"curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg",
	`echo "deb [ arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" > /etc/apt/sources.list.d/mongodb-org-8.0.list`,
}

var MongoClientModule = aptClientModule(
	types.ModuleMongoClient,
	"MongoDB client (mongosh)",
	"MONGODB CLIENT",
	mongoClientSetup,
	"mongodb-mongosh",
	&types.ContextSection{
		Title: "MongoDB client",
		Body: ctxBody(
			"`mongosh` is installed. It is only the client — the server, if any, is a",
			"sibling container described below.",
		),
	},
)
