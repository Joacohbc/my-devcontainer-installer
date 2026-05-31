// Package sshhelp prints the "next steps — SSH access" guide shown
// after a project is generated.
package sshhelp

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

func keyPath() string { return "~/.ssh/" + sshdefaults.KeyName }

func linuxBlock() string {
	prompt := ui.Subtle("   $ ")
	ipVar := "IP_SSH"
	ipCmd := fmt.Sprintf("%s=$(docker inspect -f '%s' %s | head -n1)", ipVar, sshdefaults.DockerIPFormat, sshdefaults.ServiceName)
	block, _ := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     "local",
		Alias:    sshdefaults.Alias,
		User:     sshdefaults.User,
		Key:      keyPath(),
		Hostname: "$" + ipVar,
	})
	return strings.Join([]string{
		ui.Bold("4) Install key + register host (Linux / Mac, direct container IP):"),
		prompt + ipCmd,
		prompt + fmt.Sprintf("ssh-copy-id -i %s.pub %s@$%s", keyPath(), sshdefaults.User, ipVar),
		prompt + fmt.Sprintf("cat <<EOF >> ~/.ssh/config\n\n%s\nEOF", block),
	}, "\n")
}

func windowsBlock() string {
	prompt := ui.Subtle("   $ ")
	port := domain.ResolveSSHHostPort()
	block, _ := sshdefaults.BuildConfigBlock(sshdefaults.ConfigBlockOptions{
		Mode:     "windows",
		Alias:    sshdefaults.Alias,
		User:     sshdefaults.User,
		Key:      keyPath(),
		Hostname: "localhost",
		Port:     fmt.Sprintf("%d", port),
	})
	return strings.Join([]string{
		ui.StyleBold.Render(fmt.Sprintf("4) Install key + register host (Windows / Git Bash, port %d):", port)),
		prompt + fmt.Sprintf("ssh-copy-id -p %d -i %s.pub %s@localhost", port, keyPath(), sshdefaults.User),
		prompt + fmt.Sprintf("cat <<EOF >> ~/.ssh/config\n\n%s\nEOF", block),
	}, "\n")
}

// Print writes the SSH setup guide for the given workspace.
func Print(workspace string) {
	isWindows := runtime.GOOS == "windows"
	bar := ui.Subtle(strings.Repeat("─", 64))
	prompt := ui.Subtle("   $ ")
	composeRel := fmt.Sprintf(".dc_%s/build/docker-compose.yml", workspace)
	startCmd := fmt.Sprintf("docker compose -f %s up -d", composeRel)
	passwordCmd := fmt.Sprintf("docker compose -f %s logs %s | grep \"%s password\" | tail -n 1", composeRel, sshdefaults.ServiceName, sshdefaults.User)

	platformBlock := linuxBlock()
	if isWindows {
		platformBlock = windowsBlock()
	}

	out := strings.Join([]string{
		ui.StyleHeader.Render("Next steps — SSH access"),
		bar,
		"",
		ui.Bold("1) Start the stack (if not running):"),
		prompt + startCmd,
		"",
		ui.Subtle("   Or simply run:  devcontainer-cli setup-ssh   (auto-detects compose)"),
		"",
		ui.Bold("2) Get temporary password (one-time, to install your key):"),
		prompt + passwordCmd,
		"",
		ui.StyleBold.Render(fmt.Sprintf("3) Generate SSH key (skip if you already have %s):", keyPath())),
		prompt + fmt.Sprintf("ssh-keygen -t ed25519 -f %s -N \"\" -q", keyPath()),
		"",
		platformBlock,
		"",
		ui.Bold("5) Connect:"),
		prompt + "ssh " + sshdefaults.Alias,
		"",
		ui.Subtle("For remote-server access (ProxyCommand) see the README."),
		bar,
		"",
	}, "\n")

	fmt.Println(out)
}
