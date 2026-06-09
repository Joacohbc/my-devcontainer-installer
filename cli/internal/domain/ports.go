package domain

import "strings"

// BindLoopback prefixes a docker `-p` / compose `ports:` spec with 127.0.0.1 so
// the published port is only reachable from the local machine, never the LAN. A
// spec that already carries an explicit host IP (two or more colons, e.g.
// "0.0.0.0:8080:80" or "127.0.0.1::80") is returned unchanged. The optional
// protocol suffix ("/tcp", "/udp") adds no colon, so counting colons is safe.
//
//	"8080:80"     -> "127.0.0.1:8080:80"   (host:container)
//	"80"          -> "127.0.0.1::80"       (container only, ephemeral host port)
//	"0.0.0.0:..." -> unchanged             (user-supplied IP is respected)
func BindLoopback(spec string) string {
	switch strings.Count(spec, ":") {
	case 0:
		return "127.0.0.1::" + spec
	case 1:
		return "127.0.0.1:" + spec
	default:
		return spec
	}
}
