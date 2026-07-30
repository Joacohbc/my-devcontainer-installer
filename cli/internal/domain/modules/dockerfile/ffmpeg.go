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
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "FFmpeg",
			Body:  "`ffmpeg`, `ffprobe` and `ffplay` are installed. There is no display or audio device, so `ffplay` is of no use here — encode to a file instead.",
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## FFMPEG
##
RUN apt-get update && apt-get install -y ffmpeg && %s
`, aptCleanup())
	},
}
