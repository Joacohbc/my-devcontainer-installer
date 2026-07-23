package dockerfile

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var ZellijModule = &ModuleSpec{
	ID:         types.ModuleZellij,
	Label:      "Zellij",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return `##
## ZELLIJ
##
RUN ARCH="$(dpkg --print-architecture)" && \
    case "$ARCH" in \
      amd64) ZELLIJ_ARCH="x86_64-unknown-linux-musl" ;; \
      arm64) ZELLIJ_ARCH="aarch64-unknown-linux-musl" ;; \
      *) echo "Unsupported architecture for Zellij: $ARCH" && exit 1 ;; \
    esac && \
    wget -nv -O /tmp/zellij.tar.gz "https://github.com/zellij-org/zellij/releases/latest/download/zellij-${ZELLIJ_ARCH}.tar.gz" && \
    tar -xzf /tmp/zellij.tar.gz -C /usr/local/bin zellij && \
    chmod +x /usr/local/bin/zellij && \
    rm /tmp/zellij.tar.gz && \
    su - devuser -c "mkdir -p /home/devuser/.config/zellij && printf '%s\n' 'default_shell \"zsh\"' > /home/devuser/.config/zellij/config.kdl"
`
	},
}
