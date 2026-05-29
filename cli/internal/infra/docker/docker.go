package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Runner interface {
	Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (status int, stdout, stderr string)
}

type DefaultRunner struct{}

var defaultRunner Runner = &DefaultRunner{}

func SetRunner(r Runner) { defaultRunner = r }

func ResetRunner() { defaultRunner = &DefaultRunner{} }

var (
	availCache *bool
	availMu    sync.Mutex
	globalCtx  context.Context = context.Background()
	globalMu   sync.RWMutex
)

func SetContext(ctx context.Context) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalCtx = ctx
}

func getContext() context.Context {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalCtx == nil {
		return context.Background()
	}
	return globalCtx
}

func IsDockerAvailable() bool {
	ctx := getContext()
	if ctx.Err() != nil {
		return false
	}
	availMu.Lock()
	defer availMu.Unlock()
	if availCache != nil {
		return *availCache
	}
	status, _, _ := defaultRunner.Run(
		ctx,
		[]string{"docker", "version", "--format", "{{.Client.Version}}"},
		"pipe", "", nil,
	)
	result := status == 0
	if ctx.Err() == nil {
		availCache = &result
	}
	return result
}

func ResetDockerCache() {
	availMu.Lock()
	defer availMu.Unlock()
	availCache = nil
}

func EnsureDocker() error {
	if !IsDockerAvailable() {
		return fmt.Errorf("docker is not installed or the daemon is not running")
	}
	return nil
}

func DockerInherit(args []string) (int, error) {
	if err := EnsureDocker(); err != nil {
		return 1, err
	}
	fullArgs := append([]string{"docker"}, args...)
	status, _, _ := defaultRunner.Run(getContext(), fullArgs, "inherit", "", nil)
	return status, nil
}

func DockerCapture(args []string) (status int, stdout, stderr string, err error) {
	if err = EnsureDocker(); err != nil {
		return 1, "", "", err
	}
	fullArgs := append([]string{"docker"}, args...)
	status, stdout, stderr = defaultRunner.Run(getContext(), fullArgs, "pipe", "", nil)
	return status, stdout, stderr, nil
}

type ComposeOptions struct {
	CWD string
	Env map[string]string
}

func DockerCompose(composeFile string, args []string, opts *ComposeOptions) (int, error) {
	if err := EnsureDocker(); err != nil {
		return 1, err
	}
	cmdArgs := append([]string{"docker", "compose", "-f", composeFile}, args...)
	cwd := ""
	var env map[string]string
	if opts != nil {
		cwd = opts.CWD
		env = opts.Env
	}
	status, _, _ := defaultRunner.Run(getContext(), cmdArgs, "inherit", cwd, env)
	return status, nil
}

func DockerComposeOrThrow(composeFile string, args []string, opts *ComposeOptions) error {
	status, err := DockerCompose(composeFile, args, opts)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("docker compose -f %s %s failed", composeFile, strings.Join(args, " "))
	}
	return nil
}

func (r *DefaultRunner) Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (status int, stdout, stderr string) {
	if len(args) == 0 {
		return 1, "", "no arguments provided"
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if env != nil {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	if stdio == "inherit" {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				return exitErr.ExitCode(), "", ""
			}
			return 1, "", runErr.Error()
		}
		return 0, "", ""
	}

	outBytes, runErr := cmd.Output()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), string(outBytes), string(exitErr.Stderr)
		}
		return 1, "", runErr.Error()
	}
	return 0, string(outBytes), ""
}

func DockerExecStdin(input []byte, args []string) (int, error) {
	if err := EnsureDocker(); err != nil {
		return 1, err
	}
	ctx := getContext()
	if _, ok := defaultRunner.(*DefaultRunner); ok {
		fullArgs := append([]string{"docker"}, args...)
		cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
			return 1, err
		}
		return 0, nil
	}
	// Mock fallback for testing
	fullArgs := append([]string{"docker"}, args...)
	status, _, _ := defaultRunner.Run(ctx, fullArgs, "pipe", "", nil)
	return status, nil
}
