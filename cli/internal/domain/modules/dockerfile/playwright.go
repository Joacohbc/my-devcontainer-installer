package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// PlaywrightModule installs the operating-system libraries that Playwright's
// Chromium / "Chrome headless" browser needs to run on Ubuntu 24.04. It is
// deliberately language-agnostic: it does NOT pull a Node/Python/Java client
// nor a browser binary, because the actual browser revision is pinned by the
// Playwright client the user later installs in their language of choice.
//
// With these system deps already present, the follow-up step is just (for the
// chosen language, run as devuser):
//
//	npx playwright install chromium      # Node / TypeScript
//	python -m playwright install chromium # Python
//	mvn exec:java ... "install chromium"  # Java
//
// which downloads the matching browser into the user's cache without needing
// root or apt at runtime. Firefox / WebKit engine deps (a larger, more
// release-volatile set) can be added on top with `playwright install-deps`
// once a client is present.
//
// Package names use Ubuntu 24.04 (noble) spellings, including the `t64`
// suffixes introduced by the 64-bit time_t transition (libasound2t64,
// libatk1.0-0t64, …).
var PlaywrightModule = &ModuleSpec{
	ID:         types.ModulePlaywright,
	Label:      "Playwright (Chromium headless system dependencies)",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## PLAYWRIGHT (Chromium headless system dependencies)
##
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
       libnss3 \
       libnspr4 \
       libdbus-1-3 \
       libatk1.0-0t64 \
       libatk-bridge2.0-0t64 \
       libatspi2.0-0t64 \
       libcups2t64 \
       libdrm2 \
       libgbm1 \
       libxcomposite1 \
       libxdamage1 \
       libxext6 \
       libxfixes3 \
       libxrandr2 \
       libxkbcommon0 \
       libasound2t64 \
       libcairo2 \
       libpango-1.0-0 \
       fonts-liberation \
       fonts-noto-color-emoji \
    && %s
`, aptCleanup())
	},
}
