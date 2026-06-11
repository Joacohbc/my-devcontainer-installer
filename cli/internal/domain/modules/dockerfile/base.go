package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// UbuntuLTS is the single Ubuntu LTS release every image is built on. Pinned
// (not user-selectable) so the base stays predictable across all containers.
const UbuntuLTS = "24.04"

var BaseModule = &ModuleSpec{
	ID:        types.ModuleBase,
	Label:     "Base (Ubuntu " + UbuntuLTS + " LTS + SSH + zsh + sudo)",
	Category:  types.CategoryBase,
	Always:    true,
	CopyFiles: []string{"zsh-installer.sh"},
	Render: func(opts map[string]any) string {
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
`, UbuntuLTS, aptCleanup())
	},
}
