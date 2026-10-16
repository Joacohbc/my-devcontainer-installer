package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Runner abstracts OS subprocess execution (such as exec.CommandContext) for the docker package.
// It serves as a dependency injection point: by programming against this abstraction rather than
// concrete OS execution, unit tests can swap the runner (via SetRunner) with mock/fake implementations
// to simulate Docker daemon states, mock exit codes, and capture command invocations without spawning
// real system processes.
type Runner interface {
	// Run executes a command with the given context, arguments, working directory, and environment.
	// The stdio parameter specifies stream handling ("inherit" connects streams to the terminal;
	// "pipe" captures outputs in stdout and stderr return values).
	Run(ctx context.Context, args []string, stdio string, cwd string, env map[string]string) (status int, stdout, stderr string)
}

type DefaultRunner struct{}

var defaultRunner Runner = &DefaultRunner{}

func SetRunner(r Runner) { defaultRunner = r }

func ResetRunner() { defaultRunner = &DefaultRunner{} }

var (
	availCache   *bool
	availMu      sync.Mutex
	globalCtx    context.Context = context.Background()
	globalMu     sync.RWMutex
	hostMu       sync.RWMutex
	hostOverride string
)

const engineVersionFormat = "{{.Client.Version}}"

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

// SetHostOverride points every subsequent engine invocation at a remote
// daemon reached over ssh: host is "user@host" or an existing ssh-config
// alias, rendered as ssh://<host> in the engine's own remote-host variable
// (Engine.RemoteHostVar) — the engine's ssh transport does the tunneling.
// Pair with ResetHostOverride (typically via defer).
func SetHostOverride(host string) {
	hostMu.Lock()
	defer hostMu.Unlock()
	hostOverride = host
}

func ResetHostOverride() {
	hostMu.Lock()
	defer hostMu.Unlock()
	hostOverride = ""
}

// hostOverrideEnv returns the env map to pass to Runner.Run for the current
// override, or nil when none is set (Run leaves cmd.Env untouched on nil,
// which is what every call site relied on before this existed).
func hostOverrideEnv() map[string]string {
	hostMu.RLock()
	host := hostOverride
	hostMu.RUnlock()
	if host == "" {
		return nil
	}
	return map[string]string{CurrentEngine().RemoteHostVar: "ssh://" + host}
}

func IsDockerAvailable() bool {
	ctx := getContext()
	if ctx.Err() != nil {
		return false
	}
	engine := CurrentEngine()
	availMu.Lock()
	defer availMu.Unlock()
	if availCache != nil {
		return *availCache
	}
	isAvailable := probeEngine(ctx, engine)
	if ctx.Err() == nil {
		availCache = &isAvailable
	}
	return isAvailable
}

func probeEngine(ctx context.Context, engine Engine) bool {
	status, _, _ := defaultRunner.Run(
		ctx,
		[]string{engine.Binary, "version", "--format", engineVersionFormat},
		"pipe", "", nil,
	)
	return status == 0
}

func ResetDockerCache() {
	availMu.Lock()
	defer availMu.Unlock()
	availCache = nil
}

func EnsureDocker() error {
	if IsDockerAvailable() {
		return nil
	}
	return unavailableError(CurrentEngine())
}

// DockerInherit runs a docker command inheriting the parent process's standard streams
// (Stdin, Stdout, Stderr). It is used for interactive shell sessions, real-time logging,
// or operations where terminal interaction is required. It returns the process exit status.
func DockerInherit(args []string) (int, error) {
	if err := EnsureDocker(); err != nil {
		return 1, err
	}
	fullArgs := append([]string{CurrentEngine().Binary}, args...)
	status, _, _ := defaultRunner.Run(getContext(), fullArgs, "inherit", "", hostOverrideEnv())
	return status, nil
}

// DockerCapture runs a docker command in the background, piping and collecting its standard
// output and error streams into memory strings. It is used for programmatic checks and queries
// (such as state check or JSON list processing) and does not write to the user's terminal.
// It returns the process exit status, captured stdout, captured stderr, and any execution error.
func DockerCapture(args []string) (status int, stdout, stderr string, err error) {
	if err = EnsureDocker(); err != nil {
		return 1, "", "", err
	}
	fullArgs := append([]string{CurrentEngine().Binary}, args...)
	status, stdout, stderr = defaultRunner.Run(getContext(), fullArgs, "pipe", "", hostOverrideEnv())
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
	cmdArgs := append([]string{CurrentEngine().Binary, "compose", "-f", composeFile}, args...)
	cwd := ""
	var env map[string]string
	if opts != nil {
		cwd = opts.CWD
		env = opts.Env
	}
	status, _, _ := defaultRunner.Run(getContext(), cmdArgs, "inherit", cwd, env)
	if status != 0 && IsPodman() {
		fmt.Fprintln(composeHintOutput, podmanComposeHint)
	}
	return status, nil
}

// podmanComposeHint is printed when `podman compose` fails. Unlike Docker's
// plugin, it is a thin wrapper over an external provider, and the commonest
// reason for it failing has nothing to do with the compose file: the provider
// (docker-compose) talks to the Podman API socket, which is not running unless
// something started it.
const podmanComposeHint = "hint: `podman compose` delegates to docker-compose or podman-compose. " +
	"If it could not connect to the API, start the socket (systemctl --user enable --now podman.socket), " +
	"or pick the provider with PODMAN_COMPOSE_PROVIDER=podman-compose."

// composeHintOutput is where the hint goes; a variable so tests can capture it.
var composeHintOutput io.Writer = os.Stderr

func DockerComposeOrThrow(composeFile string, args []string, opts *ComposeOptions) error {
	status, err := DockerCompose(composeFile, args, opts)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("%s compose -f %s %s failed", CurrentEngine().Name, composeFile, strings.Join(args, " "))
	}
	return nil
}

// mergeEnv layers overrides on top of the process's own environment. A key
// present in both must resolve to overrides' value: exec.Cmd does not
// guarantee a later duplicate wins over an earlier one (many getenv
// implementations return the first match), so the inherited entry is dropped
// rather than merely shadowed.
func mergeEnv(overrides map[string]string) []string {
	merged := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, overridden := overrides[k]; !overridden {
			merged = append(merged, kv)
		}
	}
	for k, v := range overrides {
		merged = append(merged, k+"="+v)
	}
	return merged
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
		cmd.Env = mergeEnv(env)
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
		fullArgs := append([]string{CurrentEngine().Binary}, args...)
		cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
		if env := hostOverrideEnv(); env != nil {
			cmd.Env = mergeEnv(env)
		}
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
	fullArgs := append([]string{CurrentEngine().Binary}, args...)
	status, _, _ := defaultRunner.Run(ctx, fullArgs, "pipe", "", hostOverrideEnv())
	return status, nil
}
