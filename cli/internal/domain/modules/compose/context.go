package compose

import "strings"

// ctxBody joins the markdown lines of a ContextSection body.
//
// Section bodies are written as double-quoted Go strings rather than raw
// backtick literals on purpose: markdown code spans are backticked, and a raw
// literal cannot contain a backtick.
func ctxBody(lines ...string) string {
	return strings.Join(lines, "\n")
}

// dbCredentials resolves the user/password a database service was rendered with,
// applying the same fallbacks as its Render so ~/CONTEXT.md never prints
// credentials that differ from the ones in the compose file.
func dbCredentials(ctx RenderContext) (user, password string) {
	user, password = ctx.DefaultDBUser, ctx.DefaultDBPassword
	if user == "" {
		user = "devuser"
	}
	if password == "" {
		password = "devpass"
	}
	return user, password
}
