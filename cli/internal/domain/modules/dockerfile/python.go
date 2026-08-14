package dockerfile

import (
	"fmt"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

var PythonModule = &ModuleSpec{
	ID:         types.ModulePython,
	Label:      "Python (python3 + pip, optional uv)",
	Category:   types.CategoryLang,
	UICategory: types.UICategoryLanguages,
	Options: []types.ModuleOption{
		{
			ID:      "uv",
			Label:   "Install uv (Astral, for devuser)",
			Type:    types.ModuleOptionConfirm,
			Default: true,
		},
	},
	// UV_SYSTEM_PYTHON makes `uv pip` target the container's own interpreter
	// without a virtualenv — the container is already the isolation boundary.
	// alias.sh exports it too, for the shells that predate this declaration;
	// here it also reaches a process started without one.
	//
	// UV_BREAK_SYSTEM_PACKAGES is what makes that target usable at all. Ubuntu
	// 24.04 marks its interpreter externally managed (PEP 668), so uv refuses to
	// write to it — `--system` alone gets `The interpreter at /usr is externally
	// managed`. Astral documents this variable as the one for CI and containers,
	// which is exactly what this is: the image is disposable, and apt is not
	// going to fight uv over a tree nothing else installs into.
	//
	// Deliberately NOT set here: VIRTUAL_ENV. A venv declared image-wide would
	// make every `uv add`/`uv sync`/`uv run` in every project print
	// "VIRTUAL_ENV=... does not match the project environment path ... and will
	// be ignored" — uv does not read it for project commands. The per-project
	// flow must stay clean, so the image-wide target is the system interpreter.
	ProvidesEnv: func(opts map[string]any) ContainerEnv {
		if !types.BoolOpt(opts, "uv", true) {
			return ContainerEnv{}
		}
		return ContainerEnv{
			Assignments: []EnvVar{
				{Name: "UV_SYSTEM_PYTHON", Value: "1"},
				{Name: "UV_BREAK_SYSTEM_PACKAGES", Value: "1"},
			},
			PathEntries: []PathEntry{"$HOME/.local/bin"},
		}
	},
	Context: func(opts map[string]any) *types.ContextSection {
		if !types.BoolOpt(opts, "uv", true) {
			return &types.ContextSection{
				Title: "Python",
				Body: ctxBody(
					"`python3` and `pip` are installed, and `pip` is the real pip here, not a",
					"wrapper: `uv` was **not** selected for this image, so none of the uv",
					"commands exist and nothing rewrites `pip` for you.",
					"",
					"Ubuntu marks the system interpreter externally managed (PEP 668), so a",
					"plain `pip install X` is refused. Create a virtualenv for the project",
					"(`python3 -m venv .venv && .venv/bin/pip install X`) — with uv absent it",
					"is the one place pip can write to. Rebuilding with the `uv` option on",
					"(`--with python`, uv defaults to enabled) is the other way out.",
				),
			}
		}
		return &types.ContextSection{
			Title: "Python — use `uv`, not `pip`",
			Body: ctxBody(
				"`uv` is the package manager in this container. It has one command per",
				"case, and picking the right one matters more than the pip muscle memory:",
				"",
				"| What you want | Command | Where it lands |",
				"|---|---|---|",
				"| A dependency of **one project** | `uv add X`, then `uv run …` | `.venv/` in the project directory |",
				"| A **command-line tool** | `uv tool install X` | its own env, on PATH via `~/.local/bin` |",
				"| A library importable **anywhere in the container** | `uv pip install X` | the system interpreter |",
				"",
				"`uv add` is the normal one for project work, and the `.venv/` it creates in",
				"the project is expected — you never create or activate it by hand. What you",
				"should not do is build a venv yourself to work around an install error.",
				"",
				"`uv pip install X` writes to the system interpreter and needs no venv or",
				"sudo: the image exports `UV_SYSTEM_PYTHON=1` and `UV_BREAK_SYSTEM_PACKAGES=1`",
				"(Ubuntu marks its interpreter externally managed, PEP 668) and gives devuser",
				"write access to the install directories. `pip` and `pip3` are shell functions",
				"forwarding here, so the old commands keep working in a shell — but they are",
				"functions, so a bare `docker exec` does not see them. Call `uv pip` directly.",
			),
		}
	},
	Render: func(opts map[string]any) string {
		uv := types.BoolOpt(opts, "uv", true)
		if !uv {
			return fmt.Sprintf(`##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip && %s
`, aptCleanup())
		}
		// python3 + pip and uv (Astral installer for devuser) install in one RUN
		// layer; cleanup runs last so the apt cache never lands in the layer.
		//
		// The chown is what makes `uv pip install X` work as devuser. UV_SYSTEM_PYTHON
		// points uv at the system interpreter, whose install directories are
		// root-owned, so without this every install dies on a permission error the
		// moment it gets past PEP 668. The paths are asked of the interpreter rather
		// than hardcoded: Debian's sysconfig uses the `posix_local` scheme, so the
		// answer is under /usr/local, and it moves with the Ubuntu release. Both
		// targets are needed — purelib for the package, scripts for any console
		// entry point it ships. The chown is not recursive on purpose: it grants
		// devuser the right to create entries in those directories without
		// rewriting the ownership of what other modules already installed there
		// (zellij drops a binary in the scripts dir).
		return fmt.Sprintf(`##
## PYTHON
##
RUN apt-get update && apt-get install -y python3 python3-pip && \
    su - devuser -c 'curl -LsSf https://astral.sh/uv/install.sh | sh' && \
    PY_PURELIB="$(python3 -c 'import sysconfig; print(sysconfig.get_path("purelib"))')" && \
    PY_SCRIPTS="$(python3 -c 'import sysconfig; print(sysconfig.get_path("scripts"))')" && \
    mkdir -p "$PY_PURELIB" "$PY_SCRIPTS" && \
    chown devuser:devuser "$PY_PURELIB" "$PY_SCRIPTS" && \
    %s
`, aptCleanup())
	},
}
