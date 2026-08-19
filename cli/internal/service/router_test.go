package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

type routerTestRunner struct {
	mu         sync.Mutex
	calls      [][]string
	running    bool
	exists     bool
	inspectOut string
	psOut      string
}

func (r *routerTestRunner) Run(_ context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, args)
	if len(args) >= 2 && args[1] == "version" {
		return 0, "27.0.0", ""
	}

	if slices.Contains(args, "ps") {
		if r.psOut != "" {
			return 0, r.psOut, ""
		}
		return 0, "myproject-devcontainer-ssh\n", ""
	}

	if slices.Contains(args, "inspect") {
		if r.inspectOut != "" {
			return 0, r.inspectOut, ""
		}
		if !r.exists {
			return 1, "", "Error: No such container"
		}
		if r.running {
			return 0, "true\n", ""
		}
		return 0, "false\n", ""
	}

	if slices.Contains(args, "run") {
		r.exists = true
		r.running = true
		return 0, "container-id-123\n", ""
	}

	if slices.Contains(args, "start") {
		r.running = true
		return 0, "", ""
	}

	if slices.Contains(args, "stop") {
		r.running = false
		return 0, "", ""
	}

	if slices.Contains(args, "rm") {
		r.exists = false
		return 0, "", ""
	}

	if slices.Contains(args, "network") {
		return 0, "", ""
	}

	return 0, "", ""
}

func (r *routerTestRunner) callContaining(token string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, call := range r.calls {
		if slices.Contains(call, token) {
			return call
		}
	}
	return nil
}

func TestEnsureRouterRunning_StartsContainerWhenNotExists(t *testing.T) {
	runner := &routerTestRunner{exists: false, running: false}
	defer useFakeDocker(runner)()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	ctx := context.Background()
	err := svc.EnsureRouterRunning(ctx)
	if err != nil {
		t.Fatalf("EnsureRouterRunning failed: %v", err)
	}

	if runner.callContaining("run") == nil {
		t.Error("expected 'docker run' to be called for devcli-router")
	}
}

func TestEnsureRouterRunning_StartsExistingStoppedContainer(t *testing.T) {
	runner := &routerTestRunner{exists: true, running: false}
	defer useFakeDocker(runner)()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	ctx := context.Background()
	err := svc.EnsureRouterRunning(ctx)
	if err != nil {
		t.Fatalf("EnsureRouterRunning failed: %v", err)
	}

	if runner.callContaining("start") == nil {
		t.Error("expected 'docker start' to be called for existing stopped container")
	}
}

func TestAddRoute_And_ListRoutes_WithMockCaddy(t *testing.T) {
	runner := &routerTestRunner{exists: true, running: true}
	defer useFakeDocker(runner)()

	var mu sync.Mutex
	routes := []domain.CaddyRoute{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(domain.BuildInitialCaddyConfig())
		case r.Method == http.MethodGet && r.URL.Path == "/config/apps/http/servers/srv0":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(domain.CaddyHTTPServer{Listen: []string{":80", ":443"}, Routes: routes})
		case r.Method == http.MethodGet && r.URL.Path == "/config/apps/http/servers/srv0/routes":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(routes)
		case r.Method == http.MethodPost && r.URL.Path == "/config/apps/http/servers/srv0/routes":
			var newRoute domain.CaddyRoute
			if err := json.NewDecoder(r.Body).Decode(&newRoute); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			routes = append(routes, newRoute)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/id/"):
			id := strings.TrimPrefix(r.URL.Path, "/id/")
			var filtered []domain.CaddyRoute
			for _, rt := range routes {
				if rt.ID != id {
					filtered = append(filtered, rt)
				}
			}
			routes = filtered
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "router_state.json")

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
		StatePath:  stateFile,
	}

	ctx := context.Background()
	rule := domain.RouteRule{
		Domain:     "api.devcli.localhost",
		TargetHost: "myproject-devcontainer-ssh",
		TargetPort: 3000,
		Workspace:  "myproject",
	}

	if err := svc.AddRoute(ctx, rule, ""); err != nil {
		t.Fatalf("AddRoute failed: %v", err)
	}

	listed, err := svc.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("ListRoutes failed: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("expected 1 route, got %d", len(listed))
	}
	if listed[0].Domain != "api.devcli.localhost" {
		t.Errorf("listed route domain = %q, want %q", listed[0].Domain, "api.devcli.localhost")
	}
	if listed[0].TargetHost != "myproject-devcontainer-ssh" || listed[0].TargetPort != 3000 {
		t.Errorf("listed route target = %s:%d, want myproject-devcontainer-ssh:3000", listed[0].TargetHost, listed[0].TargetPort)
	}

	// Remove by domain name
	if err := svc.RemoveRoute(ctx, "api.devcli.localhost"); err != nil {
		t.Fatalf("RemoveRoute by domain failed: %v", err)
	}

	afterRemove, err := svc.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("ListRoutes after remove failed: %v", err)
	}
	if len(afterRemove) != 0 {
		t.Errorf("expected 0 routes after removal, got %d", len(afterRemove))
	}
}

func TestAddRoute_RollbackOnFailure(t *testing.T) {
	runner := &routerTestRunner{exists: true, running: true}
	defer useFakeDocker(runner)()

	snapshotRestored := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"snapshot":"initial"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/config/apps/http/servers/srv0":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/config/apps/http/servers/srv0/routes":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid route structure"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/load":
			snapshotRestored = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	ctx := context.Background()
	rule := domain.RouteRule{
		Domain:     "api.devcli.localhost",
		TargetHost: "myproject-devcontainer-ssh",
		TargetPort: 3000,
		Workspace:  "myproject",
	}

	err := svc.AddRoute(ctx, rule, "")
	if err == nil {
		t.Fatal("expected AddRoute to fail on bad request")
	}

	if !snapshotRestored {
		t.Error("expected snapshot to be restored via /load on failure")
	}
}

func TestReconcileOrphans(t *testing.T) {
	// ps returns only myproject-devcontainer-ssh (orphan-app is missing)
	runner := &routerTestRunner{exists: true, running: true, psOut: "myproject-devcontainer-ssh\n"}
	defer useFakeDocker(runner)()

	var mu sync.Mutex
	routes := []domain.CaddyRoute{
		{
			ID:    "devcli_orphan_1",
			Match: []domain.CaddyRouteMatch{{Host: []string{"orphan.devcli.localhost"}}},
			Handle: []domain.CaddyReverseProxyHandler{
				{Handler: "reverse_proxy", Upstreams: []domain.CaddyUpstream{{Dial: "orphan-app:8080"}}},
			},
		},
		{
			ID:    "devcli_active_2",
			Match: []domain.CaddyRouteMatch{{Host: []string{"active.devcli.localhost"}}},
			Handle: []domain.CaddyReverseProxyHandler{
				{Handler: "reverse_proxy", Upstreams: []domain.CaddyUpstream{{Dial: "myproject-devcontainer-ssh:3000"}}},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/apps/http/servers/srv0/routes":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(routes)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/id/"):
			id := strings.TrimPrefix(r.URL.Path, "/id/")
			var filtered []domain.CaddyRoute
			for _, rt := range routes {
				if rt.ID != id {
					filtered = append(filtered, rt)
				}
			}
			routes = filtered
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	ctx := context.Background()
	if err := svc.ReconcileOrphans(ctx); err != nil {
		t.Fatalf("ReconcileOrphans failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(routes) != 1 || routes[0].ID != "devcli_active_2" {
		t.Errorf("expected only active route to remain, got %+v", routes)
	}
}

func TestVerifyUpstreamContainer(t *testing.T) {
	runnerRunning := &routerTestRunner{exists: true, running: true}
	defer useFakeDocker(runnerRunning)()

	svc := RouterService{Report: nopReporter{}}
	if err := svc.VerifyUpstreamContainer("my-container"); err != nil {
		t.Errorf("expected running container to verify, got %v", err)
	}

	runnerStopped := &routerTestRunner{exists: true, running: false}
	defer useFakeDocker(runnerStopped)()

	if err := svc.VerifyUpstreamContainer("my-container"); err == nil {
		t.Error("expected stopped container to fail verification")
	}
}

func TestRouterState_LoadAndSave(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "state.json")
	svc := RouterService{Report: nopReporter{}, StatePath: tempFile}

	initial, err := svc.LoadState()
	if err != nil {
		t.Fatalf("initial LoadState failed: %v", err)
	}
	if len(initial.Routes) != 0 {
		t.Errorf("expected empty initial state, got %d", len(initial.Routes))
	}

	testRule := domain.RouteRule{
		ID:         "test_id",
		Domain:     "test.devcli.localhost",
		TargetHost: "my-container",
		TargetPort: 3000,
	}

	if err := svc.SaveState(domain.RouterState{Routes: []domain.RouteRule{testRule}}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState after save failed: %v", err)
	}
	if len(loaded.Routes) != 1 || loaded.Routes[0].ID != "test_id" {
		t.Errorf("unexpected loaded state: %+v", loaded)
	}
}

func TestRouterStatus_RunningAndStopped(t *testing.T) {
	runnerRunning := &routerTestRunner{exists: true, running: true}
	defer useFakeDocker(runnerRunning)()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	svc := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	ctx := context.Background()
	st, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !st.Running {
		t.Error("expected status.Running to be true")
	}

	runnerStopped := &routerTestRunner{exists: true, running: false}
	defer useFakeDocker(runnerStopped)()

	svcStopped := RouterService{
		Report:     nopReporter{},
		AdminURL:   server.URL,
		HTTPClient: server.Client(),
	}

	stStopped, err := svcStopped.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if stStopped.Running {
		t.Error("expected status.Running to be false for stopped container")
	}
}

func TestStopRouter(t *testing.T) {
	runner := &routerTestRunner{exists: true, running: true}
	defer useFakeDocker(runner)()

	svc := RouterService{Report: nopReporter{}}
	if err := svc.StopRouter(); err != nil {
		t.Fatalf("StopRouter failed: %v", err)
	}

	if runner.callContaining("stop") == nil {
		t.Error("expected 'docker stop' for devcli-router")
	}
	if runner.callContaining("rm") == nil {
		t.Error("expected 'docker rm' for devcli-router")
	}
}
