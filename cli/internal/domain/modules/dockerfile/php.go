package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PhpModule = &ModuleSpec{
	ID:         types.ModulePhp,
	Label:      "PHP (with Composer, Ondřej PPA, Latest version)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Options: []types.ModuleOption{
		{
			ID:      "composer",
			Label:   "Install Composer (Dependency Manager)",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	Render: func(opts map[string]any) string {
		composer := true
		if v, ok := opts["composer"]; ok {
			if b, ok := v.(bool); ok {
				composer = b
			}
		}

		composerBlock := ""
		if composer {
			composerBlock = `
# Install Composer
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
`
		}

		return fmt.Sprintf(`##
## PHP (Latest version)
##
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends software-properties-common \
    && add-apt-repository -y ppa:ondrej/php \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
       php \
       php-cli \
       php-common \
       php-fpm \
       php-xml \
       php-zip \
       php-curl \
       php-mbstring \
       php-gd \
       php-mysql \
       php-sqlite3 \
    && usermod -aG www-data devuser && usermod -aG devuser www-data%s`,
			composerBlock,
		)
	},
}
