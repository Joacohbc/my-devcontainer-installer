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
// install, so the whole client is a single layer.
func aptClientModule(id types.ModuleID, label, title string, setupCmds []string, pkg string) *ModuleSpec {
	return &ModuleSpec{
		ID:         id,
		Label:      label,
		Category:   types.CategoryDB,
		UICategory: types.UICategoryClients,
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
	Render: func(opts map[string]any) string {
		version, _ := opts["version"].(string)
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

// MysqlClientComponents maps a pinned MySQL release to its apt repo component.
var MysqlClientComponents = map[string]string{
	"8.0": "mysql-8.0",
	"8.4": "mysql-8.4-lts",
	"9.0": "mysql-innovation",
}

var MysqlClientModule = &ModuleSpec{
	ID:         types.ModuleMysqlClient,
	Label:      "MySQL client (mysql)",
	Category:   types.CategoryDB,
	UICategory: types.UICategoryClients,
	Options: []types.ModuleOption{
		versionOption("mysql client version", []types.ModuleOptionChoice{
			{Value: "9.0", Label: "MySQL 9.0 (Innovation)"},
			{Value: "8.4", Label: "MySQL 8.4 (LTS)"},
			{Value: "8.0", Label: "MySQL 8.0 (LTS)"},
		}),
	},
	Render: func(opts map[string]any) string {
		version, _ := opts["version"].(string)
		component, ok := MysqlClientComponents[version]
		if !ok {
			// Generic, MySQL-compatible client (MariaDB) from Ubuntu's repos.
			return fmt.Sprintf(`##
## MYSQL CLIENT
##
RUN apt-get update && apt-get install -y \
    default-mysql-client && %s
`, aptCleanup())
		}
		// Pin the real Oracle MySQL client via the official MySQL apt repository.
		return fmt.Sprintf(`##
## MYSQL CLIENT (MySQL %s)
##
RUN install -m 0755 -d /etc/apt/keyrings && \
    export DEBIAN_FRONTEND=noninteractive && \
    curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu $(lsb_release -cs) %s" > /etc/apt/sources.list.d/mysql.list && \
    apt-get update && \
    apt-get install -y mysql-community-client && \
    %s
`, version, component, aptCleanup())
	},
}

var RedisClientModule = aptClientModule(
	types.ModuleRedisClient,
	"Redis client (redis-cli)",
	"REDIS CLIENT",
	nil,
	"redis-tools",
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
)
