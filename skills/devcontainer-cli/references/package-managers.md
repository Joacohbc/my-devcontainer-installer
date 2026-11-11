# Package managers inside the container

The image replaces the usual tools, and the replacements live in shell
functions and aliases — so the tool you reach for from `agent exec` is often the
wrong one. Run `devcontainer-cli inspect context` to see which of these the image has.

- **Python: `uv`, never `pip`.** uv has one command per case, and they are not
  interchangeable:

  | What you want | Command |
  |---|---|
  | A dependency of **one project** | `uv add X`, then `uv run …` |
  | A **command-line tool** | `uv tool install X` |
  | A library importable **anywhere in the container** | `uv pip install X` |

  `uv add` is the one for project work; the `.venv/` it creates in the project
  directory is expected and you never activate it by hand. `uv pip install X`
  writes to the system interpreter and needs no venv and no sudo — the image
  sets `UV_SYSTEM_PYTHON=1` and `UV_BREAK_SYSTEM_PACKAGES=1` and gives devuser
  write access to the install directories, so **do not pass
  `--break-system-packages` yourself and do not build a venv to work around an
  install error**.

  Never `pip install` or `python3 -m pip install`: the base is Ubuntu 24.04,
  whose system Python is PEP 668 "externally managed", so plain pip refuses
  with `error: externally-managed-environment`. `pip`/`pip3` are shell
  *functions* forwarding to `uv pip`, so they work in a shell but not from a
  bare `agent exec` — call `uv pip` directly. If the image was built without uv
  (`--modules python` sets the `uv` option, which can be turned off), you are on
  plain pip and a venv is then the honest answer.
- **Node: call `pnpm` directly.** `npm`/`npx` are *aliases*, and a
  non-interactive shell does not expand aliases — so `bash -lc 'npm install'`
  really runs npm, not pnpm.

      devcontainer-cli agent exec -- uv pip install requests
      devcontainer-cli agent exec -- pnpm add -D vitest
      devcontainer-cli agent exec -w -- uv add rich
