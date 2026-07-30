package compose

import (
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ctxBody joins the markdown lines of a ContextSection body.
//
// Section bodies are written as double-quoted Go strings rather than raw
// backtick literals on purpose: markdown code spans are backticked, and a raw
// literal cannot contain a backtick.
func ctxBody(lines ...string) string {
	return strings.Join(lines, "\n")
}

// dbCredentials resolves the user/password a database service runs with. Both
// Render and Context go through it, which is what guarantees ~/CONTEXT.md prints
// the very credentials written into the compose file rather than a second
// implementation of the same fallbacks.
func dbCredentials(ctx RenderContext) (user, password string) {
	user, password = ctx.DefaultDBUser, ctx.DefaultDBPassword
	if user == "" {
		user = types.FallbackDBUser
	}
	if password == "" {
		password = types.FallbackDBPassword
	}
	return user, password
}
