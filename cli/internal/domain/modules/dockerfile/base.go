package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var BaseModule = &ModuleSpec{
	ID:        types.ModuleBase,
	Label:     "Base (Ubuntu + SSH + zsh + sudo)",
	Category:  types.CategoryBase,
	Always:    true,
	CopyFiles: []string{"zsh-installer.sh"},
	Options: []types.ModuleOption{
		{
			ID:    "ubuntu",
			Label: "Ubuntu version",
			Type:  types.ModuleOptionSelect,
			Choices: []types.ModuleOptionChoice{
				{Value: "24.04", Label: "24.04 (Noble)"},
				{Value: "22.04", Label: "22.04 (Jammy)"},
			},
			Default: "24.04",
		},
	},
	Render: func(opts map[string]any) string {
		ubuntu, _ := opts["ubuntu"].(string)
		if ubuntu == "" {
			ubuntu = "24.04"
		}
		return fmt.Sprintf(`# Use an Ubuntu base image
FROM ubuntu:%s

# Update packages and install SSH, sudo, and other utilities
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    openssh-server \
    nano \
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
    git \
    wget \
    unzip \
    rsync \
    apt-transport-https \
    && %s

# Create devuser with sudo privileges
RUN useradd -m -s /bin/zsh devuser && \
    usermod -aG sudo devuser && \
    echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Install Zsh configuration and plugins
COPY zsh-installer.sh /tmp/zsh-installer.sh
RUN chmod +x /tmp/zsh-installer.sh && \
    su - devuser -c "/tmp/zsh-installer.sh" && \
    rm /tmp/zsh-installer.sh
`, ubuntu, aptCleanup())
	},
}
