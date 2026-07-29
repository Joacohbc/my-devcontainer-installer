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
	Context: func(opts map[string]any) *types.ContextSection {
		lines := []string{
			"The latest PHP from the Ondřej PPA, with the `xml`, `zip`, `curl`, `mbstring`,",
			"`gd`, `mysql` and `sqlite3` extensions plus `php-fpm`.",
		}
		if types.BoolOpt(opts, "composer", true) {
			lines = append(lines, "", "Composer is installed as `/usr/local/bin/composer`.")
		} else {
			lines = append(lines, "", "Composer is **not** installed in this image.")
		}
		return &types.ContextSection{Title: "PHP", Body: ctxBody(lines...)}
	},
	Render: func(opts map[string]any) string {
		composer := types.BoolOpt(opts, "composer", true)

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
    && usermod -aG www-data devuser && usermod -aG devuser www-data \
    && %s%s`,
			aptCleanup(), composerBlock,
		)
	},
}
