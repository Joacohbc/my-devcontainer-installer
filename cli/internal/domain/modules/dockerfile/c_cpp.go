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
