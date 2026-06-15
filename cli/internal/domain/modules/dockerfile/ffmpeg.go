package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var FfmpegModule = &ModuleSpec{
	ID:         types.ModuleFfmpeg,
	Label:      "FFmpeg",
	Category:   types.CategoryInfra,
	UICategory: types.UICategoryDevTools,
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## FFMPEG
##
RUN apt-get update && apt-get install -y ffmpeg && %s
`, aptCleanup())
	},
}
