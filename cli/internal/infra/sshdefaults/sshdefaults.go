// Package sshdefaults holds the shared SSH constants and the ssh config block
// builder used by setup-ssh and the printed SSH instructions.
package sshdefaults

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

const (
	User        = "devuser"
	ServiceName = "devcontainer-ssh"
	Alias       = "devcontainer"
	KeyName     = "id_devcontainer"
	WindowsPort = types.DefaultSSHHostPort
	// DockerIPFormat emits one IP per line so a container on several networks
	// does not concatenate addresses with no separator.
	DockerIPFormat = `{{range .NetworkSettings.Networks}}{{.IPAddress}}{{"\n"}}{{end}}`
)

func DefaultKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", KeyName)
}

func AuthorizedKeysInstallScript() string {
	return "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys && sort -u ~/.ssh/authorized_keys -o ~/.ssh/authorized_keys && chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"
}

// RemoteKeygenCommand returns the ssh-keygen invocation a user runs on the
// machine they connect FROM to create the ed25519 key pair at keyPath. It
// mirrors the flags used by the local key generation.
func RemoteKeygenCommand(keyPath string) string {
	return fmt.Sprintf(`ssh-keygen -t ed25519 -f %s -N "" -q`, keyPath)
}

// RemoteInstallKeyCommand returns the one-liner that pipes the public key at
// keyPath into the container's authorized_keys over an ssh hop to the docker
// host (remote), reusing AuthorizedKeysInstallScript so it stays in sync with
// the local install path.
func RemoteInstallKeyCommand(keyPath, remote, user, container string) string {
	return fmt.Sprintf(`cat %s.pub | ssh %s "docker exec -i -u %s %s sh -c '%s'"`,
		keyPath, remote, user, container, AuthorizedKeysInstallScript())
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

// BuildConfigBlock renders an ~/.ssh/config Host stanza for the given mode.
func BuildConfigBlock(opts ConfigBlockOptions) (string, error) {
	lines := []string{"Host " + opts.Alias}
	switch opts.Mode {
	case ModeLocal:
		if opts.Hostname == "" {
			return "", fmt.Errorf("hostname required for local mode")
		}
		lines = append(lines,
			"    HostName "+opts.Hostname,
			"    User "+opts.User,
			"    IdentityFile "+opts.KeyPath,
		)
	case ModeWindows:
		hostname := opts.Hostname
		if hostname == "" {
			hostname = "localhost"
		}
		port := opts.Port
		if port == "" {
			port = fmt.Sprintf("%d", WindowsPort)
		}
		lines = append(lines,
			"    HostName "+hostname,
			"    Port "+port,
			"    User "+opts.User,
			"    IdentityFile "+opts.KeyPath,
		)
	case ModeRemote:
		if opts.Remote == "" {
			return "", fmt.Errorf("remote required for remote mode")
		}
		if opts.Container == "" {
			return "", fmt.Errorf("container required for remote mode")
		}
		// The ProxyCommand is wrapped in double quotes, so the command
		// substitution and the template's inner quotes must be escaped: \$ keeps
		// the docker inspect running on the remote host (not the local machine),
		// and \" stops the format's quotes from closing the ProxyCommand string.
		escapedFormat := strings.ReplaceAll(DockerIPFormat, `"`, `\"`)
		ipExpr := fmt.Sprintf(`\$(docker inspect -f '%s' %s | head -n1)`, escapedFormat, opts.Container)
		lines = append(lines,
			"    User "+opts.User,
			"    IdentityFile "+opts.KeyPath,
			fmt.Sprintf(`    ProxyCommand ssh %s "nc -q0 %s 22"`, opts.Remote, ipExpr),
		)
	}
	return strings.Join(lines, "\n"), nil
}
