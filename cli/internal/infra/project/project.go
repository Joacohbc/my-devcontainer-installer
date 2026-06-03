package project

import "path/filepath"

type Paths struct {
	ProjectDir       string
	BuildDir         string
	ComposeFile      string
	DockerfilePath   string
	EnvPath          string
	ForwardStateFile string
	ForwardLogDir    string
}

func ProjectPaths(cwd, workspace string) Paths {
	projectDir := filepath.Join(cwd, ".dc_"+workspace)
	buildDir := filepath.Join(projectDir, "build")
	return Paths{
		ProjectDir:       projectDir,
		BuildDir:         buildDir,
		ComposeFile:      filepath.Join(buildDir, "docker-compose.yml"),
		DockerfilePath:   filepath.Join(buildDir, "Dockerfile"),
		EnvPath:          filepath.Join(buildDir, ".env"),
		ForwardStateFile: filepath.Join(projectDir, "port-forwards.json"),
		ForwardLogDir:    filepath.Join(projectDir, "port-forward-logs"),
	}
}
