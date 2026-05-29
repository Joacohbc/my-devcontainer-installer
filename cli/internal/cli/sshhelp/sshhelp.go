// Package sshhelp prints the "next steps — SSH access" guide shown
// after a project is generated.
package sshhelp

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

var (
	cyanBold  = color.New(color.FgCyan, color.Bold)
	boldColor = color.New(color.Bold)
	gray      = color.New(color.FgWhite)
)

func keyPath() string { return "~/.ssh/" + sshdefaults.KeyName }

func linuxBlock() string {
	prompt := gray.Sprint("   $ ")
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
		boldColor.Sprint("4) Install key + register host (Linux / Mac, direct container IP):"),
		prompt + ipCmd,
		prompt + fmt.Sprintf("ssh-copy-id -i %s.pub %s@$%s", keyPath(), sshdefaults.User, ipVar),
		prompt + fmt.Sprintf("cat <<EOF >> ~/.ssh/config\n\n%s\nEOF", block),
	}, "\n")
}

func windowsBlock() string {
	prompt := gray.Sprint("   $ ")
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
		boldColor.Sprintf("4) Install key + register host (Windows / Git Bash, port %d):", port),
		prompt + fmt.Sprintf("ssh-copy-id -p %d -i %s.pub %s@localhost", port, keyPath(), sshdefaults.User),
		prompt + fmt.Sprintf("cat <<EOF >> ~/.ssh/config\n\n%s\nEOF", block),
	}, "\n")
}

// Print writes the SSH setup guide for the given workspace.
func Print(workspace string) {
	isWindows := runtime.GOOS == "windows"
	bar := gray.Sprint(strings.Repeat("─", 64))
	prompt := gray.Sprint("   $ ")
	composeRel := fmt.Sprintf(".dc_%s/build/docker-compose.yml", workspace)
	startCmd := fmt.Sprintf("docker compose -f %s up -d", composeRel)
	passwordCmd := fmt.Sprintf("docker compose -f %s logs %s | grep \"%s password\" | tail -n 1", composeRel, sshdefaults.ServiceName, sshdefaults.User)

	platformBlock := linuxBlock()
	if isWindows {
		platformBlock = windowsBlock()
	}

	out := strings.Join([]string{
		cyanBold.Sprint("Next steps — SSH access"),
		bar,
		"",
		boldColor.Sprint("1) Start the stack (if not running):"),
		prompt + startCmd,
		"",
		gray.Sprint("   Or simply run:  devcontainer-cli setup-ssh   (auto-detects compose)"),
		"",
		boldColor.Sprint("2) Get temporary password (one-time, to install your key):"),
		prompt + passwordCmd,
		"",
		boldColor.Sprintf("3) Generate SSH key (skip if you already have %s):", keyPath()),
		prompt + fmt.Sprintf("ssh-keygen -t ed25519 -f %s -N \"\" -q", keyPath()),
		"",
		platformBlock,
		"",
		boldColor.Sprint("5) Connect:"),
		prompt + "ssh " + sshdefaults.Alias,
		"",
		gray.Sprint("For remote-server access (ProxyCommand) see the README."),
		bar,
		"",
	}, "\n")

	fmt.Println(out)
}
