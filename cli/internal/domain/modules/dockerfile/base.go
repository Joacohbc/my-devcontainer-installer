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
    su - devuser -c "/tmp/zsh-installer.sh" && \
    rm /tmp/zsh-installer.sh
`, UbuntuLTS, aptCleanup())
	},
}
