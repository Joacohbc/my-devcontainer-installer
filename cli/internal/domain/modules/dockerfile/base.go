package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// UbuntuLTS is the single Ubuntu LTS release every image is built on. Pinned
// (not user-selectable) so the base stays predictable across all containers.
const UbuntuLTS = "24.04"

// p10kStyles are the Powerlevel10k presets shipped upstream under
// powerlevel10k/config/p10k-<style>.zsh. "none" leaves p10k unconfigured, so
// the container drops into the interactive `p10k configure` wizard on first
// login.
var p10kStyles = map[string]bool{
	"none":    true,
	"lean":    true,
	"classic": true,
	"rainbow": true,
	"pure":    true,
}

// DefaultP10kStyle is the style baked in when the option is absent or holds an
// unrecognized value. It is deliberately a real preset rather than "none": the
// remote images are generated with --no-interactive, which never fills option
// defaults in, so a "none" fallback shipped every prebuilt image unconfigured
// and dropped first-time users into the `p10k configure` wizard. Opting out is
// still possible by selecting "none" explicitly.
const DefaultP10kStyle = "lean"

var BaseModule = &ModuleSpec{
	ID:        types.ModuleBase,
	Label:     "Base (Ubuntu " + UbuntuLTS + " LTS + SSH + zsh + sudo)",
	Category:  types.CategoryBase,
	Always:    true,
	CopyFiles: []string{"zsh-installer.sh", "setup-help.sh"},
	// ~/.local/bin holds whatever the post-script installers (Claude Code,
	// Antigravity, …) and pip/uv --user drop, so it is declared by the base
	// rather than by any single language module.
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		return ContainerEnv{PathEntries: []PathEntry{"$HOME/.local/bin"}}
	},
	Options: []types.ModuleOption{
		{
			ID:    "p10kStyle",
			Label: "Powerlevel10k default style",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "none", Label: "None (run 'p10k configure' manually)"},
				{Value: "lean", Label: "Lean"},
				{Value: "classic", Label: "Classic"},
				{Value: "rainbow", Label: "Rainbow"},
				{Value: "pure", Label: "Pure"},
			},
			Default: DefaultP10kStyle,
		},
	},
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "Base image",
			Body: ctxBody(
				"Ubuntu "+UbuntuLTS+" LTS. The login shell is zsh (oh-my-zsh + powerlevel10k).",
				"PATH and the toolchain variables are baked into the image environment, so a",
				"process started without any shell (`docker exec <binary>`) already has them;",
				"they are also in `~/"+EnvScriptFile+"`, sourced from `.zshenv`, `.profile` and",
				"`.bashrc`. Aliases and functions are the exception — those need a shell.",
				"",
				"Preinstalled: `git`, `curl`, `wget`, `jq`, `unzip`, `lsof`, plus the `micro` and",
				"`nano` terminal editors. `cat ~/help` prints a micro/zellij keyboard",
				"cheat-sheet.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		p10kStyle := types.StringOpt(opts, "p10kStyle", "")
		if !p10kStyles[p10kStyle] {
			p10kStyle = DefaultP10kStyle
		}
		zshInstallerCmd := "/tmp/zsh-installer.sh"
		if p10kStyle != "none" {
			zshInstallerCmd += " " + p10kStyle
		}
		return fmt.Sprintf(`# Use an Ubuntu base image
FROM ubuntu:%s

# Update packages and install SSH, sudo, and other utilities
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    openssh-server \
    nano \
    micro \
    sudo \
    pwgen \
    zsh \
    fontconfig \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    acl \
    jq \
    lsof \
    git \
    wget \
    unzip \
    apt-transport-https \
    && %s

# Keepalive so long-lived ssh sessions (e.g. a --via jump through a NAT/
# firewall) aren't silently dropped as idle, and a session whose peer vanished
# without closing it is reclaimed server-side: probe every 60s, drop after 3
# unanswered probes (180s). Drop any existing directive (active or commented)
# first so this is idempotent and always wins regardless of line order.
RUN sed -i '/^#\?\s*ClientAliveInterval/d; /^#\?\s*ClientAliveCountMax/d' /etc/ssh/sshd_config && \
    printf 'ClientAliveInterval 60\nClientAliveCountMax 3\n' >> /etc/ssh/sshd_config

# Create devuser with sudo privileges. USER_UID/USER_GID are build args so a
# local-cached image bakes the host owner of the bind-mounted workspace and the
# container never has to renumber the user at runtime. Remote prebuilt images
# leave the 1000 default. Ubuntu >= 23.10 ships a stock "ubuntu" user/group on
# UID/GID 1000, removed here so the target ids are free and devuser owns them.
ARG USER_UID=1000
ARG USER_GID=1000
RUN userdel -r ubuntu 2>/dev/null || true; \
    groupdel ubuntu 2>/dev/null || true; \
    existing_group="$(getent group "${USER_GID}" | cut -d: -f1)"; \
    if [ -z "$existing_group" ]; then \
        groupadd -g "${USER_GID}" devuser; \
    elif [ "$existing_group" != "devuser" ]; then \
        groupmod -n devuser "$existing_group"; \
    fi; \
    useradd -m -u "${USER_UID}" -g "${USER_GID}" -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Install Zsh configuration and plugins
COPY zsh-installer.sh /tmp/zsh-installer.sh
RUN chmod +x /tmp/zsh-installer.sh && \
    su - devuser -c "%s" && \
    rm /tmp/zsh-installer.sh

# Install the ~/help quick reference (micro + zellij shortcuts) into devuser's
# home so every container ships the cheat-sheet. Written as devuser so it is
# owned by them, then the installer script is removed in the same layer.
COPY setup-help.sh /tmp/setup-help.sh
RUN chmod +x /tmp/setup-help.sh && \
    su - devuser -c "/tmp/setup-help.sh" && \
    rm /tmp/setup-help.sh
`, UbuntuLTS, aptCleanup(), zshInstallerCmd)
	},
}
