package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
)

type RouterStatus struct {
	Running       bool     `json:"running"`
	ContainerName string   `json:"container_name"`
	AdminURL      string   `json:"admin_url"`
	HTTPPort      int      `json:"http_port"`
	HTTPSPort     int      `json:"https_port"`
	Networks      []string `json:"networks"`
	ActiveRoutes  int      `json:"active_routes"`
}

type RouterService struct {
	Report     Reporter
	AdminURL   string
	HTTPClient *http.Client
	StatePath  string
}

func (s RouterService) adminURL() string {
	if s.AdminURL != "" {
		return strings.TrimRight(s.AdminURL, "/")
	}
	return fmt.Sprintf("http://127.0.0.1:%d", domain.RouterAdminPort)
}

func (s RouterService) client() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 3 * time.Second}
}

func (s RouterService) stateFilePath() string {
	if s.StatePath != "" {
		return s.StatePath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devcontainer-cli", "router_state.json")
}

func (s RouterService) LoadState() (domain.RouterState, error) {
	path := s.stateFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.RouterState{Routes: []domain.RouteRule{}}, nil
		}
		return domain.RouterState{}, err
	}

	var state domain.RouterState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.RouterState{Routes: []domain.RouteRule{}}, nil
	}
	return state, nil
}

func (s RouterService) SaveState(state domain.RouterState) error {
	path := s.stateFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s RouterService) EnsureDocker() error {
	return docker.EnsureDocker()
}

func (s RouterService) CheckHostPortsAvailable() error {
	for _, port := range []int{domain.RouterHTTPPort, domain.RouterHTTPSPort, domain.RouterAdminPort} {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			continue
		}
		if errors.Is(err, syscall.EADDRINUSE) || strings.Contains(err.Error(), "address already in use") {
			return fmt.Errorf("host port %d is already in use by another process", port)
		}
	}
	return nil
}

func (s RouterService) EnsureRouterRunning(ctx context.Context) error {
	if err := s.EnsureDocker(); err != nil {
		return err
	}

	running, exists, err := s.inspectRouterContainer()
	if err != nil {
		return err
	}

	if exists && running {
		return s.waitForAdminAPI(ctx)
	}

	if exists && !running {
		s.Report.Info("Starting existing router container '%s'...", domain.RouterContainerName)
		status, startErr := docker.DockerInherit([]string{"start", domain.RouterContainerName})
		if startErr != nil {
			return startErr
		}
		if status != 0 {
			return fmt.Errorf("failed to start container '%s'", domain.RouterContainerName)
		}
		return s.waitForAdminAPI(ctx)
	}

	if portErr := s.CheckHostPortsAvailable(); portErr != nil {
		s.Report.Warn("Port pre-flight check warning: %v", portErr)
	}

	s.Report.Info("Starting global Caddy router container '%s'...", domain.RouterContainerName)
	args := []string{
		"run", "-d",
		"--name", domain.RouterContainerName,
		"--label", types.LabelManaged + "=true",
		"-v", domain.RouterDataVolumeName + ":/data",
		"-v", domain.RouterConfigVolumeName + ":/config",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", domain.RouterHTTPPort, domain.RouterHTTPPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", domain.RouterHTTPSPort, domain.RouterHTTPSPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", domain.RouterAdminPort, domain.RouterAdminPort),
		domain.RouterImage,
		"/bin/sh", "-c",
		`mkdir -p /config/caddy && if [ ! -f /config/caddy/autosave.json ] || ! grep -q "0.0.0.0:2019" /config/caddy/autosave.json; then echo '{"admin":{"listen":"0.0.0.0:2019","enforce_origin":false},"apps":{"http":{"servers":{"srv0":{"listen":[":80",":443"],"routes":[]}}}}}' > /config/caddy/autosave.json; fi; exec caddy run --config /config/caddy/autosave.json --resume`,
	}

	status, runErr := docker.DockerInherit(args)
	if runErr != nil {
		return runErr
	}
	if status != 0 {
		return fmt.Errorf("failed to create router container '%s'", domain.RouterContainerName)
	}

	return s.waitForAdminAPI(ctx)
}

func (s RouterService) inspectRouterContainer() (running bool, exists bool, err error) {
	status, stdout, _, cerr := docker.DockerCapture([]string{
		"inspect",
		"--format", "{{.State.Running}}",
		domain.RouterContainerName,
	})
	if cerr != nil || status != 0 {
		return false, false, nil
	}

	output := strings.TrimSpace(stdout)
	if output == "true" {
		return true, true, nil
	}
	if output == "false" {
		return false, true, nil
	}
	return false, false, nil
}

func (s RouterService) waitForAdminAPI(ctx context.Context) error {
	deadline := time.Now().Add(5 * time.Second)
	reqURL := s.adminURL() + "/config/"
	delay := 50 * time.Millisecond

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err == nil {
			resp, rerr := s.client().Do(req)
			if rerr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
					return s.ensureBaseServerInitialized(ctx)
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			if delay < 500*time.Millisecond {
				delay *= 2
			}
		}
	}

	return fmt.Errorf("timed out waiting for Caddy Admin API at %s", reqURL)
}

func (s RouterService) ensureBaseServerInitialized(ctx context.Context) error {
	reqURL := s.adminURL() + "/config/apps/http/servers/srv0"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	initialServer := domain.CaddyHTTPServer{
		Listen: []string{":80", ":443"},
		Routes: []domain.CaddyRoute{},
	}
	payload, err := json.Marshal(initialServer)
	if err != nil {
		return err
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := s.client().Do(putReq)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		body, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("failed to initialize Caddy HTTP server (status %d): %s", putResp.StatusCode, string(body))
	}

	return nil
}

func (s RouterService) VerifyUpstreamContainer(targetHost string) error {
	status, stdout, _, err := docker.DockerCapture([]string{
		"inspect",
		"--format", "{{.State.Running}}",
		targetHost,
	})
	if err != nil || status != 0 {
		return fmt.Errorf("target container %q was not found", targetHost)
	}

	if strings.TrimSpace(stdout) != "true" {
		return fmt.Errorf("target container %q is not running", targetHost)
	}

	return nil
}

func (s RouterService) isRouterConnectedToNetwork(networkName string) bool {
	status, stdout, _, err := docker.DockerCapture([]string{
		"inspect",
		"--format", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
		domain.RouterContainerName,
	})
	if err != nil || status != 0 {
		return false
	}
	for _, n := range strings.Fields(stdout) {
		if n == networkName {
			return true
		}
	}
	return false
}

func (s RouterService) ConnectWorkspaceNetwork(workspace string) (string, error) {
	netSvc := NetworkService{Report: s.Report}
	networkName, err := netSvc.ResolveNetwork(workspace)
	if err != nil {
		return "", err
	}

	if s.isRouterConnectedToNetwork(networkName) {
		return networkName, nil
	}

	connectStatus, cerr := docker.DockerInherit([]string{
		"network", "connect",
		networkName,
		domain.RouterContainerName,
	})
	if cerr != nil {
		return "", cerr
	}
	if connectStatus != 0 {
		return "", fmt.Errorf("failed to connect router to network %q", networkName)
	}

	return networkName, nil
}

func (s RouterService) ConnectContainerNetwork(targetHost string) (string, error) {
	status, stdout, _, err := docker.DockerCapture([]string{
		"inspect",
		"--format", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
		targetHost,
	})
	if err != nil || status != 0 {
		return "", fmt.Errorf("failed to inspect networks for container %q", targetHost)
	}

	networks := strings.Fields(strings.TrimSpace(stdout))
	if len(networks) == 0 {
		return "", fmt.Errorf("container %q is not attached to any network", targetHost)
	}

	targetNetwork := networks[0]
	for _, n := range networks {
		if n != "bridge" && n != "host" && n != "none" {
			targetNetwork = n
			break
		}
	}

	if s.isRouterConnectedToNetwork(targetNetwork) {
		return targetNetwork, nil
	}

	connectStatus, cerr := docker.DockerInherit([]string{
		"network", "connect",
		targetNetwork,
		domain.RouterContainerName,
	})
	if cerr != nil {
		return "", cerr
	}
	if connectStatus != 0 {
		return "", fmt.Errorf("failed to connect router to network %q", targetNetwork)
	}

	return targetNetwork, nil
}

func (s RouterService) CreateSnapshot(ctx context.Context) ([]byte, error) {
	reqURL := s.adminURL() + "/config/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Caddy configuration snapshot (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (s RouterService) RestoreSnapshot(ctx context.Context, snapshot []byte) error {
	if len(snapshot) == 0 {
		return nil
	}

	reqURL := s.adminURL() + "/load"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(snapshot))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to restore Caddy snapshot (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s RouterService) AddRoute(ctx context.Context, rule domain.RouteRule, workspace string) error {
	if err := domain.ValidateRouteRule(rule); err != nil {
		return err
	}

	if err := s.EnsureRouterRunning(ctx); err != nil {
		return err
	}

	if err := s.VerifyUpstreamContainer(rule.TargetHost); err != nil {
		s.Report.Warn("Upstream verification: %v", err)
	}

	if rule.TargetHost != "" {
		if _, err := s.ConnectContainerNetwork(rule.TargetHost); err != nil && workspace != "" {
			_, _ = s.ConnectWorkspaceNetwork(workspace)
		}
	} else if workspace != "" {
		_, _ = s.ConnectWorkspaceNetwork(workspace)
	}

	snapshot, err := s.CreateSnapshot(ctx)
	if err != nil {
		s.Report.Warn("Could not take configuration snapshot: %v", err)
	}

	caddyRoute, err := domain.BuildCaddyRoute(rule)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(caddyRoute)
	if err != nil {
		return err
	}

	// First ensure clean state by removing any route with the same ID
	_ = s.RemoveRoute(ctx, caddyRoute.ID)

	routesURL := s.adminURL() + "/config/apps/http/servers/srv0/routes"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, routesURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		if len(snapshot) > 0 {
			_ = s.RestoreSnapshot(ctx, snapshot)
		}
		return fmt.Errorf("failed to register route in Caddy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if len(snapshot) > 0 {
			_ = s.RestoreSnapshot(ctx, snapshot)
		}
		return fmt.Errorf("Caddy rejected route (status %d): %s", resp.StatusCode, string(body))
	}

	// Update persistent state
	state, _ := s.LoadState()
	state.Routes = append(state.Routes, rule)
	_ = s.SaveState(state)

	return nil
}

func (s RouterService) RemoveRoute(ctx context.Context, identifier string) error {
	cleaned := strings.TrimSpace(identifier)
	if cleaned == "" {
		return nil
	}

	targetID := cleaned
	routes, _ := s.ListRoutes(ctx)
	for _, r := range routes {
		if r.Domain == cleaned || r.ID == cleaned {
			targetID = r.ID
			break
		}
	}

	reqURL := s.adminURL() + "/id/" + targetID
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client().Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}

	state, _ := s.LoadState()
	var remaining []domain.RouteRule
	for _, r := range state.Routes {
		if r.ID != targetID && r.Domain != cleaned {
			remaining = append(remaining, r)
		}
	}
	state.Routes = remaining
	_ = s.SaveState(state)

	return nil
}

func (s RouterService) ListRoutes(ctx context.Context) ([]domain.RouteRule, error) {
	running, exists, _ := s.inspectRouterContainer()
	if !exists || !running {
		return nil, nil
	}

	reqURL := s.adminURL() + "/config/apps/http/servers/srv0/routes"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch routes from Caddy (status %d)", resp.StatusCode)
	}

	var routes []domain.CaddyRoute
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, err
	}

	var rules []domain.RouteRule
	for _, r := range routes {
		domainName := ""
		if len(r.Match) > 0 && len(r.Match[0].Host) > 0 {
			domainName = r.Match[0].Host[0]
		}

		if len(r.Handle) == 0 || len(r.Handle[0].Upstreams) == 0 {
			continue
		}

		dial := r.Handle[0].Upstreams[0].Dial
		host, portStr, err := net.SplitHostPort(dial)
		if err != nil {
			continue
		}
		port, _ := strconv.Atoi(portStr)

		rules = append(rules, domain.RouteRule{
			ID:         r.ID,
			Domain:     domainName,
			TargetHost: host,
			TargetPort: port,
		})
	}

	return rules, nil
}

func (s RouterService) ReconcileOrphans(ctx context.Context) error {
	routes, err := s.ListRoutes(ctx)
	if err != nil || len(routes) == 0 {
		return nil
	}

	status, stdout, _, cerr := docker.DockerCapture([]string{
		"ps",
		"--format", "{{.Names}}",
	})
	if cerr != nil || status != 0 {
		return nil
	}

	runningContainers := make(map[string]bool)
	for _, name := range strings.Fields(stdout) {
		runningContainers[name] = true
	}

	for _, r := range routes {
		if r.TargetHost != "" && !runningContainers[r.TargetHost] {
			s.Report.Info("Pruning orphaned route %s (target %s is stopped)", r.Domain, r.TargetHost)
			_ = s.RemoveRoute(ctx, r.ID)
		}
	}

	return nil
}

func (s RouterService) Status(ctx context.Context) (RouterStatus, error) {
	running, exists, _ := s.inspectRouterContainer()
	status := RouterStatus{
		Running:       running && exists,
		ContainerName: domain.RouterContainerName,
		AdminURL:      s.adminURL(),
		HTTPPort:      domain.RouterHTTPPort,
		HTTPSPort:     domain.RouterHTTPSPort,
	}

	if !status.Running {
		return status, nil
	}

	_ = s.ReconcileOrphans(ctx)
	routes, _ := s.ListRoutes(ctx)
	status.ActiveRoutes = len(routes)

	netStatus, stdout, _, _ := docker.DockerCapture([]string{
		"inspect",
		"--format", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
		domain.RouterContainerName,
	})
	if netStatus == 0 {
		status.Networks = strings.Fields(strings.TrimSpace(stdout))
	}

	return status, nil
}

func (s RouterService) StopRouter() error {
	s.Report.Info("Stopping global router container '%s'...", domain.RouterContainerName)
	_, _ = docker.DockerInherit([]string{"stop", domain.RouterContainerName})
	status, err := docker.DockerInherit([]string{"rm", domain.RouterContainerName})
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("failed to remove router container")
	}
	s.Report.Success("✓ Global router container stopped and removed.")
	return nil
}

func (s RouterService) RunForeground(ctx context.Context, rule domain.RouteRule, workspace string) error {
	if err := s.AddRoute(ctx, rule, workspace); err != nil {
		return err
	}

	s.Report.Success("✔ Route active: http://%s -> %s:%d", rule.Domain, rule.TargetHost, rule.TargetPort)
	s.Report.Info("Press Ctrl+C to terminate the route.\n")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case <-ctx.Done():
	case <-sigChan:
		s.Report.Info("\nClosing route %s...", rule.Domain)
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = s.RemoveRoute(cleanupCtx, rule.ID)
	s.Report.Success("✓ Route %s removed cleanly.", rule.Domain)
	return nil
}
