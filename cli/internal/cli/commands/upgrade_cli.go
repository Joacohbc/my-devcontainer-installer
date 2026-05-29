package commands

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/spf13/cobra"
)

const (
	selfUpdateRepo   = "Joacohbc/my-devcontainer-installer"
	selfUpdateUA     = "devcontainer-cli"
	primaryAPIHost   = "api.github.com"
	maxDownloadBytes = 200 * 1024 * 1024
	maxChecksumBytes = 1024
)

var allowedHostSuffixes = []string{".github.com", ".githubusercontent.com"}
var allowedHostsExact = map[string]bool{"github.com": true, "api.github.com": true}

func init() { register(newUpgradeCliCommand()) }

func newUpgradeCliCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade-cli",
		Short: "Replace this binary with the latest GitHub release",
		Long: `devcontainer-cli upgrade-cli — replace the current binary with the latest release

Env:
  GITHUB_TOKEN  Optional, to avoid the 60 req/hour anonymous rate limit`,
		SilenceUsage: true,
		RunE:         runSelfUpdate,
	}
	cmd.Flags().Bool("check", false, "Print current vs latest and exit (no download)")
	cmd.Flags().Bool("force", false, "Reinstall even if the current version matches latest")
	return cmd
}

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func getTargetTriplet() (triplet, ext string, err error) {
	var osName string
	switch runtime.GOOS {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	var archName string
	switch runtime.GOARCH {
	case "arm64":
		archName = "arm64"
	case "amd64":
		archName = "x64"
	default:
		return "", "", fmt.Errorf("unsupported arch: %s", runtime.GOARCH)
	}
	if osName == "windows" {
		ext = ".exe"
	}
	return osName + "-" + archName, ext, nil
}

func splitVersion(s string) (main []string, pre string, hasPre bool) {
	noV := strings.TrimPrefix(s, "v")
	if i := strings.Index(noV, "-"); i != -1 {
		return strings.Split(noV[:i], "."), noV[i+1:], true
	}
	return strings.Split(noV, "."), "", false
}

func compareVersions(a, b string) int {
	am, ap, ahas := splitVersion(a)
	bm, bp, bhas := splitVersion(b)
	n := len(am)
	if len(bm) > n {
		n = len(bm)
	}
	for i := 0; i < n; i++ {
		ai, bi := "0", "0"
		if i < len(am) {
			ai = am[i]
		}
		if i < len(bm) {
			bi = bm[i]
		}
		an, aerr := strconv.Atoi(ai)
		bn, berr := strconv.Atoi(bi)
		if aerr == nil && berr == nil {
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		} else if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	if !ahas && !bhas {
		return 0
	}
	if !ahas {
		return 1
	}
	if !bhas {
		return -1
	}
	if ap == bp {
		return 0
	}
	if ap < bp {
		return -1
	}
	return 1
}

func isAllowedHost(host string) bool {
	if allowedHostsExact[host] {
		return true
	}
	for _, s := range allowedHostSuffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

func checkURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("refusing non-https URL: %s", raw)
	}
	if !isAllowedHost(u.Host) {
		return nil, fmt.Errorf("host not in allowlist: %s", u.Host)
	}
	return u, nil
}

// newGitHubRequest builds a GET request and an HTTP client that re-validate the
// allowlist on every redirect hop. Shared by the JSON API calls and the
// streaming binary download so the host allowlist is enforced uniformly.
func newGitHubRequest(rawURL string, withAuth bool) (*http.Request, *http.Client, error) {
	if _, err := checkURL(rawURL); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", selfUpdateUA)
	req.Header.Set("Accept", "application/vnd.github+json")
	if withAuth {
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	client := &http.Client{
		CheckRedirect: func(r *http.Request, _ []*http.Request) error {
			_, err := checkURL(r.URL.String())
			return err
		},
	}
	return req, client, nil
}

func httpGet(rawURL string, withAuth bool, maxBytes int64) (int, []byte, error) {
	req, client, err := newGitHubRequest(rawURL, withAuth)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if int64(len(body)) > maxBytes {
		return resp.StatusCode, nil, fmt.Errorf("response exceeded %d bytes", maxBytes)
	}
	return resp.StatusCode, body, nil
}

func fetchLatestRelease() (*release, error) {
	apiURL := fmt.Sprintf("https://%s/repos/%s/releases/latest", primaryAPIHost, selfUpdateRepo)
	status, body, err := httpGet(apiURL, true, 5*1024*1024)
	if err != nil {
		return nil, err
	}
	if status == 403 {
		return nil, fmt.Errorf("GitHub API rate-limited (403). Set GITHUB_TOKEN to authenticate")
	}
	if status != 200 {
		return nil, fmt.Errorf("GitHub API %d: %s", status, string(body[:min(200, len(body))]))
	}
	var rel release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub release JSON: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("unexpected GitHub release payload")
	}
	return &rel, nil
}

func resolveAssetURL(rel *release, triplet, ext string) (binary, checksum string, err error) {
	name := fmt.Sprintf("devcontainer-cli-%s%s", triplet, ext)
	var bURL string
	for _, a := range rel.Assets {
		if a.Name == name {
			bURL = a.DownloadURL
		}
	}
	if bURL == "" {
		var names []string
		for _, a := range rel.Assets {
			names = append(names, a.Name)
		}
		return "", "", fmt.Errorf("asset %s not found in release %s. Available: %s", name, rel.TagName, strings.Join(names, ", "))
	}
	checksumName := name + ".sha256"
	var cURL string
	for _, a := range rel.Assets {
		if a.Name == checksumName {
			cURL = a.DownloadURL
		}
	}
	if cURL == "" {
		return "", "", fmt.Errorf("checksum %s not found in release %s. Refusing to update without integrity verification", checksumName, rel.TagName)
	}
	return bURL, cURL, nil
}

// humanBytes formats a byte count as a compact human-readable size.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// progressWriter is an io.Writer that tallies bytes copied through it and
// renders a single-line progress indicator to out, throttled to avoid spamming.
type progressWriter struct {
	total      int64
	written    int64
	out        io.Writer
	lastReport time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if now := time.Now(); now.Sub(p.lastReport) >= 100*time.Millisecond {
		p.lastReport = now
		p.render()
	}
	return n, nil
}

func (p *progressWriter) render() {
	if p.total > 0 {
		pct := float64(p.written) / float64(p.total) * 100
		fmt.Fprintf(p.out, "\r  downloading... %5.1f%% (%s / %s)", pct, humanBytes(p.written), humanBytes(p.total))
		return
	}
	fmt.Fprintf(p.out, "\r  downloading... %s", humanBytes(p.written))
}

func (p *progressWriter) finish() {
	p.render()
	fmt.Fprintln(p.out)
}

func downloadToFile(rawURL, dest string) error {
	req, client, err := newGitHubRequest(rawURL, false)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d %s", resp.StatusCode, rawURL)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	pw := &progressWriter{total: resp.ContentLength, out: os.Stderr}
	limited := io.LimitReader(resp.Body, maxDownloadBytes+1)
	written, err := io.Copy(f, io.TeeReader(limited, pw))
	pw.finish()
	if err != nil {
		return err
	}
	if written > maxDownloadBytes {
		return fmt.Errorf("response exceeded %d bytes", maxDownloadBytes)
	}
	return nil
}

func verifyChecksum(filePath, checksumURL string) error {
	status, body, err := httpGet(checksumURL, false, maxChecksumBytes)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("failed to fetch checksum: HTTP %d %s", status, checksumURL)
	}
	expected := strings.Fields(strings.TrimSpace(string(body)))
	if len(expected) == 0 || len(expected[0]) != 64 {
		return fmt.Errorf("invalid sha256 checksum content")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expected[0])), []byte(actual)) != 1 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", strings.ToLower(expected[0]), actual)
	}
	return nil
}

func replaceBinary(tmpPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	return swapBinary(execPath, tmpPath, runtime.GOOS == "windows")
}

// swapBinary replaces the binary at execPath with the one at tmpPath. On
// Windows the running exe cannot be overwritten, so the original is renamed
// aside to <exe>.old first; if the move-in of the new binary then fails, the
// original is rolled back into place so the user is never left without a
// working binary. On Unix the replacement is a single atomic rename, which
// already leaves the original untouched on failure.
func swapBinary(execPath, tmpPath string, windows bool) error {
	if windows {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, execPath); err != nil {
			if rbErr := os.Rename(oldPath, execPath); rbErr != nil {
				return fmt.Errorf("update failed (%v) and rollback failed (%v); restore manually from %s", err, rbErr, oldPath)
			}
			return fmt.Errorf("update failed, original binary restored: %w", err)
		}
		return nil
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpPath, execPath)
}

// CleanupStaleUpdate removes a leftover <exe>.old from a previous Windows
// in-place update. No-op on other platforms.
func CleanupStaleUpdate() {
	if runtime.GOOS != "windows" {
		return
	}
	if execPath, err := os.Executable(); err == nil {
		_ = os.Remove(execPath + ".old")
	}
}

func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	check, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")

	fmt.Fprintln(os.Stderr, ui.Subtle("Note: 'devcontainer-cli update' now manages container images. Self-update lives at 'devcontainer-cli upgrade-cli'."))

	current := version
	fmt.Printf(ui.Subtle("Current version: %s\n"), current)
	fmt.Println(ui.Subtle("Fetching latest release..."))
	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}
	latest := rel.TagName
	fmt.Printf(ui.Subtle("Latest version : %s\n"), latest)

	cmp := -1
	if current != "dev" {
		cmp = compareVersions(current, latest)
	}
	if check {
		switch {
		case cmp < 0:
			ui.Yellow("Update available: %s → %s", current, latest)
		case cmp == 0:
			ui.Green("Already up to date.")
		default:
			fmt.Println(ui.Subtle("Current version is ahead of latest release."))
		}
		return nil
	}

	if cmp >= 0 && !force {
		ui.Green("Already up to date.")
		return nil
	}

	triplet, ext, err := getTargetTriplet()
	if err != nil {
		return err
	}
	binaryURL, checksumURL, err := resolveAssetURL(rel, triplet, ext)
	if err != nil {
		return err
	}
	fmt.Printf(ui.Subtle("Downloading %s\n"), binaryURL)

	execPath, _ := os.Executable()
	tmpDir, err := os.MkdirTemp(filepath.Dir(execPath), ".devcontainer-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "binary")

	if err := downloadToFile(binaryURL, tmpPath); err != nil {
		return err
	}
	if info, serr := os.Stat(tmpPath); serr != nil || info.Size() == 0 {
		return fmt.Errorf("downloaded file is empty")
	}
	fmt.Println(ui.Subtle("Verifying checksum..."))
	if err := verifyChecksum(tmpPath, checksumURL); err != nil {
		return err
	}
	if err := replaceBinary(tmpPath); err != nil {
		return err
	}

	ui.Green("Updated to %s. Restart any running session to use the new binary.", latest)
	return nil
}
