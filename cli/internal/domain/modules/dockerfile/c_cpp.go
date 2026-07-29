package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var CCppModule = &ModuleSpec{
	ID:         types.ModuleCCpp,
	Label:      "C / C++ (GCC, Clang, CMake, GDB, build-essential)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Context: func(opts map[string]any) *types.ContextSection {
		return &types.ContextSection{
			Title: "C / C++",
			Body: ctxBody(
				"`gcc` and `clang` are both installed, with `cmake`, `ninja`, `pkg-config`,",
				"`gdb`, `clang-format`, `clang-tidy` and `valgrind`.",
				"",
				"`gdb` and `valgrind` need `--cap-add=SYS_PTRACE` to attach to a running",
				"process; the container is not started with it by default, so debugging a",
				"process you did not launch yourself will fail.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		return fmt.Sprintf(`##
## C / C++
##
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
       build-essential \
       gdb \
       cmake \
       ninja-build \
       pkg-config \
       clang \
       clang-format \
       clang-tidy \
       valgrind \
    && %s
`, aptCleanup())
	},
}
