package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ChromeModule installs Google Chrome (stable) as a standalone system browser,
// independent of any automation framework. A Playwright / Selenium / Puppeteer
// client can be added later in the user's language of choice and pointed at
// this system binary (e.g. Playwright `channel: "chrome"`,
// Selenium chromedriver), so the browser is decoupled from a
// framework-bundled Chromium.
//
// google-chrome-stable is a real .deb, so apt resolves every runtime library it
// needs — no manual dependency list. Google only publishes Chrome for amd64, so
// the install is guarded to fail fast on other architectures (there is no
// official arm64 Chrome; Chromium would be the substitute, which a Chrome-only
// module deliberately excludes).
var ChromeModule = &ModuleSpec{
	ID:         types.ModuleChrome,
	Label:      "Google Chrome (headless-capable, amd64 only)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## GOOGLE CHROME
##
RUN ARCH="$(dpkg --print-architecture)" && \
    if [ "$ARCH" != "amd64" ]; then \
      echo "Google Chrome is only published for amd64 (got: $ARCH)" && exit 1; \
    fi && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor -o /etc/apt/keyrings/google-chrome.gpg && \
    echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list && \
    apt-get update && \
    apt-get install -y google-chrome-stable && \
    %s
`, aptCleanup())
	},
}
