package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// aptClientModule builds a ModuleSpec for a database client installed from apt.
// Each client is its own selectable module under the "clients" UI category so
// they can be picked individually in the wizard. setup is optional extra RUN
// lines (e.g. adding a vendor apt repo) emitted before the install.
func aptClientModule(id, label, title, setup, pkg string) *ModuleSpec {
	return &ModuleSpec{
		ID:         id,
		Label:      label,
		Category:   types.CategoryDB,
		UICategory: types.UICategoryClients,
		Render: func(opts map[string]any) string {
			return fmt.Sprintf(`##
## %s
##
%sRUN apt-get update && apt-get install -y \
    %s
`, title, setup, pkg)
		},
	}
}

var PostgresClientModule = aptClientModule(
	"postgres-client",
	"PostgreSQL client (psql)",
	"POSTGRESQL CLIENT",
	"",
	"postgresql-client",
)

var RedisClientModule = aptClientModule(
	"redis-client",
	"Redis client (redis-cli)",
	"REDIS CLIENT",
	"",
	"redis-tools",
)

var MysqlClientModule = aptClientModule(
	"mysql-client",
	"MySQL client (mysql)",
	"MYSQL CLIENT",
	"",
	"default-mysql-client",
)

const mongoClientSetup = `RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg && \
    echo "deb [ arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-8.0.list
`

var MongoClientModule = aptClientModule(
	"mongo-client",
	"MongoDB client (mongosh)",
	"MONGODB CLIENT",
	mongoClientSetup,
	"mongodb-mongosh",
)
