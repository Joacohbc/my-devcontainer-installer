package project

import "path/filepath"

type Paths struct {
	ProjectDir     string
	BuildDir       string
	ComposeFile    string
	DockerfilePath string
	EnvPath        string
	// ContextPath is the generated CONTEXT.md the Dockerfile COPYs into the
	// image. It sits in the build dir alongside the embedded helper scripts
	// because it is a build input, not an output the user edits.
	ContextPath string
}

func ProjectPaths(cwd, workspace string) Paths {
	projectDir := filepath.Join(cwd, ".dc_"+workspace)
	buildDir := filepath.Join(projectDir, "build")
	return Paths{
		ProjectDir:     projectDir,
		BuildDir:       buildDir,
		ComposeFile:    filepath.Join(buildDir, "docker-compose.yml"),
		DockerfilePath: filepath.Join(buildDir, "Dockerfile"),
		EnvPath:        filepath.Join(buildDir, ".env"),
		ContextPath:    filepath.Join(buildDir, "CONTEXT.md"),
	}
}
