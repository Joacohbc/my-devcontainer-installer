package domain

import (
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// GenerateContext renders ~/CONTEXT.md — the orientation document baked into the
// image and read first by an AI coding agent working inside the container.
//
// The document is assembled from the modules and services this project actually
// selected: a static preamble that holds for every container, then one section
// per resolved Dockerfile module and compose service that declares a Context.
// Everything is resolved here, at build time, so the file an agent reads never
// promises a tool that was not installed or a database that is not running.
//
// It returns "" for remote builds, mirroring GenerateDockerfile: those images
// are prebuilt elsewhere and ship the CONTEXT.md of the module set they were
// built with, so there is nothing for this project to COPY.
func GenerateContext(config *types.DevcontainerConfig) (string, error) {
	if config.Mode == types.BuildModeRemote {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(types.GeneratedContextHeader)
	b.WriteString("\n\n# Container context\n\n")
	b.WriteString(contextPreamble(config))

	moduleSections, err := moduleContextSections(config)
	if err != nil {
		return "", err
	}
	for _, sec := range moduleSections {
		writeContextSection(&b, sec)
	}

	for _, sec := range serviceContextSections(config) {
		writeContextSection(&b, sec)
	}

	for _, sec := range skillContextSections(config) {
		writeContextSection(&b, sec)
	}

	writeContextSection(&b, customScriptsContextSection(config))

	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

// customScriptsContextSection tells the agent which of the scripts in this
// image are the user's own and what already ran, so it does not re-run a build
// script or wait for a manual one to have happened by itself. Nil when the
// project brought no scripts, which is the common case.
func customScriptsContextSection(config *types.DevcontainerConfig) *types.ContextSection {
	build, start, manual, err := PartitionCustomScripts(config)
	if err != nil || len(build)+len(start)+len(manual) == 0 {
		return nil
	}

	var lines []string
	lines = append(lines, "This project ships scripts of its own, added through its profile or `--script`.\n")
	if len(build) > 0 {
		lines = append(lines, "Already run when the image was built (do not run them again):\n")
		for _, f := range build {
			lines = append(lines, "- `"+f+"`")
		}
		lines = append(lines, "")
	}
	if len(start) > 0 {
		lines = append(lines, "Run once per container on start, by the entrypoint (logs under `~/.post-script-state`):\n")
		for _, f := range start {
			lines = append(lines, "- `"+types.PostScriptDir+"/"+f+"`")
		}
		lines = append(lines, "")
	}
	if len(manual) > 0 {
		lines = append(lines, "Copied in but never run automatically — run one yourself if you need it:\n")
		for _, f := range manual {
			lines = append(lines, "- `"+types.PostScriptDir+"/"+f+"`")
		}
	}
	return &types.ContextSection{Title: "Project scripts", Body: strings.Join(lines, "\n")}
}

// contextPreamble is the part that is true of every container the CLI builds,
// with the one value that does vary — the workspace mount point — substituted.
func contextPreamble(config *types.DevcontainerConfig) string {
	workspaceDir := types.WorkspaceDir(config.Workspace)
	return fmt.Sprintf(`You are running **inside a Docker container** managed by `+"`devcontainer-cli`"+`.
The conventions here are not the same as on a normal host, so read this before
running commands.

This file is generated from the modules and services this container was built
with, so it describes what is actually installed. For the live inventory —
versions, running services, mounted volumes — run:

    get-devcontainer-context

## Where you are

- The project is bind-mounted at `+"`%s`"+`, also reachable as the shorter
  alias `+"`%s`"+`. **Edit code only there.** Every project has its own
  path under both roots — there is no bare `+"`/workspace`"+`.
- You are the `+"`devuser`"+` user with passwordless `+"`sudo`"+`. Installing system
  packages with `+"`sudo apt-get install`"+` is fine and expected.
- **Everything outside `+"`%s`"+` and `+"`/home/devuser`"+` is discarded**
  when the container is recreated. Never leave work there.
- There is no systemd and no init system. Do not use `+"`systemctl`"+` or
  `+"`service`"+`.

`, workspaceDir, types.WorkspaceAlias(config.Workspace), workspaceDir)
}

// moduleContextSections collects the context entry of every resolved Dockerfile
// module, in the same order the modules are emitted into the Dockerfile (base
// first, then infra, languages, runtimes, database clients).
func moduleContextSections(config *types.DevcontainerConfig) ([]*types.ContextSection, error) {
	resolved, err := ResolveDockerfileModules(MatchDBClientVersions(config))
	if err != nil {
		return nil, err
	}
	var out []*types.ContextSection
	for _, r := range resolved {
		if r.Module.Context == nil {
			continue
		}
		if sec := r.Module.Context(r.Options); sec != nil {
			out = append(out, sec)
		}
	}
	return out, nil
}

// serviceContextSections collects the context entry of every enabled compose
// service, in catalog order (not the alphabetical order the compose document
// uses) so the devcontainer itself comes before the databases it talks to.
func serviceContextSections(config *types.DevcontainerConfig) []*types.ContextSection {
	resolved := resolveEnabledServices(config)
	enabled := make(map[string]bool, len(resolved.enabledIDs))
	for _, id := range resolved.enabledIDs {
		enabled[id] = true
	}

	dbUser, dbPassword := ResolveDBCredentials()

	var out []*types.ContextSection
	for _, svc := range catalog.ComposeServices {
		if !enabled[string(svc.ID)] || svc.Context == nil {
			continue
		}
		opts := resolved.optionsByID[string(svc.ID)]
		if opts == nil {
			opts = map[string]any{}
		}
		rc := compose.RenderContext{
			ImageName:         config.Image,
			Options:           opts,
			DefaultDBUser:     dbUser,
			DefaultDBPassword: dbPassword,
		}
		if svc.ID == types.ServiceDevcontainer {
			rc.Ports = devcontainerPorts(config)
			rc.WorkspaceDir = types.WorkspaceDir(config.Workspace)
			if types.SharedConfigEnabled(config) {
				rc.SharedConfigMount = types.SharedConfigMount()
			}
		}
		if sec := svc.Context(rc); sec != nil {
			out = append(out, sec)
		}
	}
	return out
}

func writeContextSection(b *strings.Builder, sec *types.ContextSection) {
	// A nil section is how a contributor says it has nothing to add for this
	// config, exactly like an empty body.
	if sec == nil {
		return
	}
	body := strings.Trim(sec.Body, "\n")
	if body == "" {
		return
	}
	b.WriteString("## " + sec.Title + "\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
}

// skillContextSections are the entries of the skills this project installs, so
// an agent reads what its own skills cover without having to look them up.
func skillContextSections(config *types.DevcontainerConfig) []*types.ContextSection {
	var sections []*types.ContextSection
	for _, id := range config.Skills.Skills {
		spec := catalog.GetAgentSkill(id)
		if spec == nil || spec.Context == nil {
			continue
		}
		sections = append(sections, spec.Context())
	}
	return sections
}
