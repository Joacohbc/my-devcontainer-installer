package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ChromeModule installs a headless-capable Chromium system browser, independent
// of any automation framework. A Playwright / Selenium / Puppeteer client can be
// added later in the user's language of choice and pointed at this system binary
// (e.g. Playwright `executablePath`, Selenium chromedriver), so the browser is
// decoupled from a framework-bundled Chromium.
//
// We install the same container-friendly Chromium .deb (binary: chromium) on
// every architecture, from the xtradeb/apps PPA. This keeps the binary name and
// behaviour identical across amd64 and arm64 — Ubuntu's own chromium-browser is
// a snap shim that cannot install during a docker build, and Google's
// google-chrome is amd64-only, so neither works everywhere.
var ChromeModule = &ModuleSpec{
	ID:         types.ModuleChrome,
	Label:      "Chromium (headless-capable, cross-arch)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## CHROMIUM
##
RUN export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get install -y --no-install-recommends software-properties-common && \
    add-apt-repository -y ppa:xtradeb/apps && \
    apt-get update && \
    apt-get install -y chromium && \
    apt-get purge -y software-properties-common && \
    %s
`, aptCleanup())
	},
}
