// Package sshdefaults holds the shared SSH constants and the ssh config block
// builder used by setup-ssh and the printed SSH instructions.
package sshdefaults

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

const (
	User        = "devuser"
	ServiceName = "devcontainer-ssh"
	Alias       = "devcontainer"
	KeyName     = types.SSHKeyName
	WindowsPort = types.DefaultSSHHostPort
	// DockerIPFormat emits one IP per line so a container on several networks
	// does not concatenate addresses with no separator.
	DockerIPFormat = `{{range .NetworkSettings.Networks}}{{.IPAddress}}{{"\n"}}{{end}}`
)

func AuthorizedKeysInstallScript() string {
	return "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && sort -u ~/.ssh/authorized_keys -o ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"
}

// RemoteExportOptions carries everything RemoteExportScript embeds into the
// self-contained snippet pasted on the connecting machine. The key material is
// passed in as bytes (no I/O in this package); the caller reads the managed key.
type RemoteExportOptions struct {
	// PrivateKey is the raw private key written to ~/.ssh/<KeyName> (chmod 600).
	PrivateKey []byte
	// PublicKey is the raw public key written to ~/.ssh/<KeyName>.pub (chmod 644).
	PublicKey []byte
	// ConfigBlock is the rendered ~/.ssh/config Host stanza (from BuildConfigBlock)
	// appended to the connecting machine's ~/.ssh/config.
	ConfigBlock string
}

// RemoteExportScript renders one self-contained `sh` snippet for the connecting
// machine. It creates ~/.ssh (0700), writes the shared private key to
// ~/.ssh/<KeyName> (chmod 600) and the public key to ~/.ssh/<KeyName>.pub
// (chmod 644), then appends the ProxyCommand Host block to ~/.ssh/config. The
// key material and config block are embedded via quoted heredocs (no expansion),
// so this is a string-only renderer with no I/O; the snippet hands over a
// PRIVATE key and must be treated as sensitive.
func RemoteExportScript(opts RemoteExportOptions) string {
	keyPath := "~/.ssh/" + KeyName
	var b strings.Builder
	b.WriteString("mkdir -p ~/.ssh && chmod 700 ~/.ssh\n")
	b.WriteString(heredoc("> "+keyPath, "DC_KEY", string(opts.PrivateKey)))
	fmt.Fprintf(&b, "chmod 600 %s\n", keyPath)
	b.WriteString(heredoc("> "+keyPath+".pub", "DC_PUB", string(opts.PublicKey)))
	fmt.Fprintf(&b, "chmod 644 %s.pub\n", keyPath)
	b.WriteString(heredoc(">> ~/.ssh/config", "DC_CFG", "\n"+opts.ConfigBlock))
	return strings.TrimRight(b.String(), "\n")
}

// heredoc renders a `cat <redirect> <<'TAG' … TAG` block with body written
// verbatim (the quoted tag disables shell expansion). A trailing newline is
// ensured so the closing tag sits on its own line.
func heredoc(redirect, tag, body string) string {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return fmt.Sprintf("cat %s <<'%s'\n%s%s\n", redirect, tag, body, tag)
}

// Mode selects how the connecting machine reaches the container, which decides
// the ~/.ssh/config stanza BuildConfigBlock renders. The type — not a comment —
// enumerates the valid values.
type Mode string

const (
	// ModeLocal: the container is reachable by its IP from the same host running
	// the CLI (HostName = container IP).
	ModeLocal Mode = "local"
	// ModeWindows: the container is reached over a forwarded port on localhost
	// (HostName = localhost, Port = forwarded port).
	ModeWindows Mode = "windows"
	// ModeRemote: the container lives on a remote docker host, reached via an ssh
	// hop to that host (ProxyCommand).
	ModeRemote Mode = "remote"
)

// ConfigBlockOptions carries everything BuildConfigBlock needs to render one
// ~/.ssh/config Host stanza. Which fields are required depends on Mode.
type ConfigBlockOptions struct {
	// Mode picks the stanza shape and therefore which fields below are consumed.
	Mode Mode
	// Alias is the connection name: the value after "Host " and what `ssh <alias>`
	// resolves to.
	Alias string
	// User is the login user inside the container (rendered as "User"; e.g. devuser).
	User string
	// KeyPath is the path to the private key, rendered as "IdentityFile".
	KeyPath string
	// Hostname is the target for "HostName": required in ModeLocal (the container
	// IP); in ModeWindows it defaults to "localhost"; unused in ModeRemote.
	Hostname string
	// Port is the host port rendered as "Port"; ModeWindows only, defaulting to
	// WindowsPort when empty.
	Port string
	// Remote is the "USER@HOST" of the docker host used in the ProxyCommand ssh
	// hop; required in ModeRemote, unused otherwise.
	Remote string
	// Container is the container name inspected for its IP inside the ProxyCommand;
	// required in ModeRemote, unused otherwise.
	Container string
}

// BuildConfigBlock renders an ~/.ssh/config Host stanza for the given mode. Each
// case returns the whole stanza as one literal so it reads like the file it
// produces; defaults and validation are applied just before the Sprintf.
func BuildConfigBlock(opts ConfigBlockOptions) (string, error) {
	switch opts.Mode {
	case ModeLocal:
		if opts.Hostname == "" {
			return "", fmt.Errorf("hostname required for local mode")
		}
		return fmt.Sprintf(`Host %s
    HostName %s
    User %s
    IdentityFile %s
    IdentitiesOnly yes`, opts.Alias, opts.Hostname, opts.User, opts.KeyPath), nil

	case ModeWindows:
		hostname := opts.Hostname
		if hostname == "" {
			hostname = "localhost"
		}
		port := opts.Port
		if port == "" {
			port = fmt.Sprintf("%d", WindowsPort)
		}
		return fmt.Sprintf(`Host %s
    HostName %s
    Port %s
    User %s
    IdentityFile %s
    IdentitiesOnly yes`, opts.Alias, hostname, port, opts.User, opts.KeyPath), nil

	case ModeRemote:
		if opts.Remote == "" {
			return "", fmt.Errorf("remote required for remote mode")
		}
		if opts.Container == "" {
			return "", fmt.Errorf("container required for remote mode")
		}
		// The ProxyCommand is wrapped in double quotes, so the command
		// substitution and the format's inner quotes must be escaped: \$ keeps the
		// docker inspect running on the remote host (not the local machine), and \"
		// stops the format's quotes from closing the ProxyCommand string. The IP
		// format itself carries Go-template-looking {{ }} braces, which is why this
		// stays plain Sprintf — feeding it through text/template would collide.
		escapedFormat := strings.ReplaceAll(DockerIPFormat, `"`, `\"`)
		ipExpr := fmt.Sprintf(`\$(docker inspect -f '%s' %s | head -n1)`, escapedFormat, opts.Container)
		return fmt.Sprintf(`Host %s
    User %s
    IdentityFile %s
    IdentitiesOnly yes
    ProxyCommand ssh %s "nc -q0 %s 22"`, opts.Alias, opts.User, opts.KeyPath, opts.Remote, ipExpr), nil
	}
	return "", fmt.Errorf("unknown mode %q", opts.Mode)
}
