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

type ConfigBlockOptions struct {
	Mode      string // "local" | "windows" | "remote"
	Alias     string
	User      string
	Key       string
	Hostname  string
	Port      string
	Remote    string
	Container string
}

// BuildConfigBlock renders an ~/.ssh/config Host stanza for the given mode.
func BuildConfigBlock(opts ConfigBlockOptions) (string, error) {
	lines := []string{"Host " + opts.Alias}
	switch opts.Mode {
	case "local":
		if opts.Hostname == "" {
			return "", fmt.Errorf("hostname required for local mode")
		}
		lines = append(lines,
			"    HostName "+opts.Hostname,
			"    User "+opts.User,
			"    IdentityFile "+opts.Key,
		)
	case "windows":
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
			"    IdentityFile "+opts.Key,
		)
	case "remote":
		if opts.Remote == "" {
			return "", fmt.Errorf("remote required for remote mode")
		}
		if opts.Container == "" {
			return "", fmt.Errorf("container required for remote mode")
		}
		ipExpr := fmt.Sprintf("$(docker inspect -f '%s' %s | head -n1)", DockerIPFormat, opts.Container)
		lines = append(lines,
			"    User "+opts.User,
			"    IdentityFile "+opts.Key,
			fmt.Sprintf(`    ProxyCommand ssh %s "nc -q0 %s 22"`, opts.Remote, ipExpr),
		)
	}
	return strings.Join(lines, "\n"), nil
}
