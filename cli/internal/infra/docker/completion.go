package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultCompletionTimeout        = 5 * time.Second
	defaultCompletionFreshTTL       = 3 * time.Minute
	defaultCompletionStaleTTL       = 30 * time.Minute
	defaultCompletionRefreshTimeout = 10 * time.Second
	completionRefreshLockTTL        = 15 * time.Second
	completionRefreshEnv            = "DC_COMPLETION_REFRESH"
	completionRefreshHostEnv        = "DC_COMPLETION_HOST"
	sshCompletionEnv                = "DC_SSH_COMPLETION"
)

var (
	completionMu          sync.RWMutex
	completionActive      bool
	completionCacheDir    string
	completionTimeout     = defaultCompletionTimeout
	completionFreshTTL    = defaultCompletionFreshTTL
	completionStaleTTL    = defaultCompletionStaleTTL
	completionNow         = time.Now
	completionRefreshHook func(host string, args []string)
)

type completionCacheEntry struct {
	UpdatedAt time.Time `json:"updated_at"`
	Stdout    string    `json:"stdout"`
}

// SetCompletionCacheDir overrides the base cache directory (empty restores os.UserCacheDir).
func SetCompletionCacheDir(dir string) {
	completionMu.Lock()
	defer completionMu.Unlock()
	completionCacheDir = dir
}

// SetCompletionTimeout overrides the per-call completion timeout (<= 0 restores default).
func SetCompletionTimeout(d time.Duration) {
	completionMu.Lock()
	defer completionMu.Unlock()
	if d <= 0 {
		completionTimeout = defaultCompletionTimeout
		return
	}
	completionTimeout = d
}

// SetCompletionNow overrides the clock used for cache TTL checks (nil restores time.Now).
func SetCompletionNow(fn func() time.Time) {
	completionMu.Lock()
	defer completionMu.Unlock()
	if fn == nil {
		completionNow = time.Now
		return
	}
	completionNow = fn
}

// SetCompletionRefreshHook overrides background cache refresh for tests (nil restores default).
func SetCompletionRefreshHook(fn func(host string, args []string)) {
	completionMu.Lock()
	defer completionMu.Unlock()
	completionRefreshHook = fn
}

func isCompletionActive() bool {
	completionMu.RLock()
	defer completionMu.RUnlock()
	return completionActive
}

func getCompletionTimeout() time.Duration {
	completionMu.RLock()
	defer completionMu.RUnlock()
	return completionTimeout
}

func getCompletionNow() time.Time {
	completionMu.RLock()
	fn := completionNow
	completionMu.RUnlock()
	return fn()
}

func currentHostOverride() string {
	hostMu.RLock()
	defer hostMu.RUnlock()
	return hostOverride
}

// WithRemoteCompletion runs fn in shell-completion mode: DockerCapture skips
// EnsureDocker, applies a short timeout, uses non-interactive SSH flags
// (BatchMode=yes, ConnectTimeout=4), and caches successful remote responses per
// (host, args) with stale-while-revalidate.
func WithRemoteCompletion(host string, fn func() error) error {
	completionMu.Lock()
	prev := completionActive
	completionActive = true
	completionMu.Unlock()
	defer func() {
		completionMu.Lock()
		completionActive = prev
		completionMu.Unlock()
	}()

	if host == "" {
		return fn()
	}
	SetHostOverride(host)
	defer ResetHostOverride()
	return fn()
}

// RunCompletionRefreshIfRequested handles a detached background cache refresh
// process spawned by stale-while-revalidate. Returns true when handled.
func RunCompletionRefreshIfRequested(ctx context.Context) bool {
	if os.Getenv(completionRefreshEnv) != "1" {
		return false
	}
	host := os.Getenv(completionRefreshHostEnv)
	args := os.Args[1:]
	if host == "" || len(args) == 0 {
		return true
	}

	refreshCtx, cancel := context.WithTimeout(ctx, defaultCompletionRefreshTimeout)
	defer cancel()
	fullArgs := append([]string{"docker"}, args...)
	status, stdout, _ := defaultRunner.Run(refreshCtx, fullArgs, "pipe", "", completionEnv(host))
	if status == 0 {
		writeCompletionCache(host, args, stdout)
	}
	removeCompletionRefreshLock(host, args)
	return true
}

func lookupCompletionCache(host string, args []string) (string, bool) {
	if host == "" {
		return "", false
	}
	entry, age, ok := readCompletionCache(host, args)
	if !ok || age > completionStaleTTL {
		return "", false
	}
	if age > completionFreshTTL {
		triggerCompletionRefresh(host, args)
	}
	return entry.Stdout, true
}

func dockerCaptureCompletion(args []string) (int, string, string, error) {
	host := currentHostOverride()
	if cached, ok := lookupCompletionCache(host, args); ok {
		return 0, cached, "", nil
	}

	ctx, cancel := context.WithTimeout(getContext(), getCompletionTimeout())
	defer cancel()
	fullArgs := append([]string{"docker"}, args...)
	status, stdout, stderr := defaultRunner.Run(ctx, fullArgs, "pipe", "", completionEnv(host))
	if host != "" && status == 0 {
		writeCompletionCache(host, args, stdout)
	} else if host != "" && ctx.Err() == context.DeadlineExceeded {
		triggerCompletionRefresh(host, args)
	}
	return status, stdout, stderr, nil
}

func completionEnv(host string) map[string]string {
	if host == "" {
		return nil
	}
	env := hostEnvFor(host)
	if env == nil {
		env = make(map[string]string, 3)
	}
	env[sshCompletionEnv] = "1"
	if binDir, err := ensureSSHWrapper(); err == nil && binDir != "" {
		if curPath := os.Getenv("PATH"); curPath != "" {
			env["PATH"] = binDir + string(os.PathListSeparator) + curPath
		} else {
			env["PATH"] = binDir
		}
	}
	return env
}

// PrewarmRemoteCompletion triggers a non-blocking background refresh for
// (host, args) when the cache is missing or past the fresh TTL.
func PrewarmRemoteCompletion(host string, args []string) {
	if host == "" || len(args) == 0 {
		return
	}
	if _, age, ok := readCompletionCache(host, args); ok && age <= completionFreshTTL {
		return
	}
	triggerCompletionRefresh(host, args)
}

func resolveCompletionBaseDir() (string, error) {
	completionMu.RLock()
	override := completionCacheDir
	completionMu.RUnlock()
	if override != "" {
		return filepath.Join(override, "devcontainer-cli"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "devcontainer-cli"), nil
}

func completionCachePath(host string, args []string) (string, error) {
	baseDir, err := resolveCompletionBaseDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(host + "\x00" + strings.Join(args, "\x00")))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(baseDir, "completions", name), nil
}

func readCompletionCache(host string, args []string) (completionCacheEntry, time.Duration, bool) {
	path, err := completionCachePath(host, args)
	if err != nil {
		return completionCacheEntry{}, 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return completionCacheEntry{}, 0, false
	}
	var entry completionCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return completionCacheEntry{}, 0, false
	}
	age := getCompletionNow().Sub(entry.UpdatedAt)
	if age < 0 {
		age = 0
	}
	return entry, age, true
}

func writeCompletionCache(host string, args []string, stdout string) {
	path, err := completionCachePath(host, args)
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	payload, err := json.Marshal(completionCacheEntry{
		UpdatedAt: getCompletionNow(),
		Stdout:    stdout,
	})
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	_ = os.Chmod(tmpName, 0o600)
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
	}
}

func triggerCompletionRefresh(host string, args []string) {
	completionMu.RLock()
	hook := completionRefreshHook
	completionMu.RUnlock()
	if hook != nil {
		hook(host, args)
		return
	}
	if _, ok := defaultRunner.(*DefaultRunner); !ok {
		return
	}
	if !acquireCompletionRefreshLock(host, args) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		removeCompletionRefreshLock(host, args)
		return
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = mergeEnv(map[string]string{
		completionRefreshEnv:     "1",
		completionRefreshHostEnv: host,
	})
	if err := cmd.Start(); err != nil {
		removeCompletionRefreshLock(host, args)
		return
	}
	_ = cmd.Process.Release()
}

func acquireCompletionRefreshLock(host string, args []string) bool {
	path, err := completionCachePath(host, args)
	if err != nil {
		return false
	}
	lockPath := path + ".lock"
	if info, statErr := os.Stat(lockPath); statErr == nil {
		if getCompletionNow().Sub(info.ModTime()) < completionRefreshLockTTL {
			return false
		}
		_ = os.Remove(lockPath)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return false
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func removeCompletionRefreshLock(host string, args []string) {
	path, err := completionCachePath(host, args)
	if err != nil {
		return
	}
	_ = os.Remove(path + ".lock")
}

// ensureSSHWrapper writes a lightweight ssh wrapper into the CLI's cache dir so
// remote completion probes fail fast without interactive prompts.
func ensureSSHWrapper() (string, error) {
	baseDir, err := resolveCompletionBaseDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(baseDir, "ssh-bin")
	realSSH, err := findRealSSH(binDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		return "", err
	}

	script := fmt.Sprintf("#!/bin/sh\nexec %q -o BatchMode=yes -o ConnectTimeout=4 \"$@\"\n", realSSH)

	wrapperPath := filepath.Join(binDir, "ssh")
	if existing, err := os.ReadFile(wrapperPath); err == nil && string(existing) == script {
		return binDir, nil
	}
	if err := os.WriteFile(wrapperPath, []byte(script), 0o700); err != nil {
		return "", err
	}
	return binDir, nil
}

func findRealSSH(excludeDir string) (string, error) {
	pathEnv := os.Getenv("PATH")
	var filtered []string
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir != "" && filepath.Clean(dir) != filepath.Clean(excludeDir) {
			filtered = append(filtered, dir)
		}
	}
	for _, dir := range filtered {
		candidate := filepath.Join(dir, "ssh")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ssh binary not found on PATH")
}
