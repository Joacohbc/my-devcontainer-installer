package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newRouteCommand()) }

func newRouteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage local domain routing for devcontainers via global Caddy proxy",
		Long: `devcontainer-cli route — dynamically map local subdomains (<name>.devcli.localhost)
directly to devcontainer services over Docker's internal network using a global Caddy router.

Subcommands:
  route add <port>       Route a domain to a container port in foreground (cleans up on Ctrl+C)
  route ls               List active domain routes
  route rm <id-or-domain> Remove a route manually
  route status           Show status of the global Caddy router container
  route stop             Stop and remove the global Caddy router container`,
		Example: `  # Route http://myproject-3000.devcli.localhost -> container:3000
  devcontainer-cli route add 3000

  # Route custom subdomain http://api.devcli.localhost -> container:8000
  devcontainer-cli route add 8000 --domain api

  # List active routes
  devcontainer-cli route ls

  # Check router status
  devcontainer-cli route status`,
		SilenceUsage: true,
	}

	cmd.AddCommand(newRouteAddCommand())
	cmd.AddCommand(newRouteListCommand())
	cmd.AddCommand(newRouteRemoveCommand())
	cmd.AddCommand(newRouteStatusCommand())
	cmd.AddCommand(newRouteStopCommand())
	return cmd
}

func newRouteAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <port>",
		Short: "Route a domain to a container port in foreground",
		Long: `devcontainer-cli route add — start routing a local domain to a port inside the
running devcontainer. The route stays active in the foreground and is cleanly
unregistered upon pressing Ctrl+C.

If --domain is omitted, it defaults to <workspace>-<port>.devcli.localhost.
If a short name is passed (e.g. --domain api), it becomes api.devcli.localhost.`,
		Example: `  devcontainer-cli route add 3000
  devcontainer-cli route add 8000 --domain api
  devcontainer-cli route add 5173 --domain frontend.devcli.localhost`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runRouteAdd,
	}

	cmd.Flags().String("domain", "", "Subdomain to route (e.g. 'api' -> 'api.devcli.localhost')")
	addContainerFlag(cmd)
	return cmd
}

func runRouteAdd(cmd *cobra.Command, args []string) error {
	portStr := strings.TrimSpace(args[0])
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %q: must be between 1 and 65535", portStr)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}
	workspace := resolveWorkspace(cwd)

	targetHost, _ := cmd.Flags().GetString("container")
	if strings.TrimSpace(targetHost) == "" {
		targetHost, _ = resolveContainer(cmd)
		if targetHost == "" {
			targetHost = fmt.Sprintf("%s-devcontainer-ssh", workspace)
		}
	}

	domainFlag, _ := cmd.Flags().GetString("domain")
	normalizedDomain := domain.NormalizeRouteDomain(domainFlag, workspace, port)

	rule := domain.RouteRule{
		ID:         domain.BuildRouteID(workspace, normalizedDomain),
		Domain:     normalizedDomain,
		TargetHost: targetHost,
		TargetPort: port,
		Workspace:  workspace,
	}

	svc := service.RouterService{Report: console}
	return svc.RunForeground(cmd.Context(), rule, workspace)
}

func newRouteListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "ls",
		Short:        "List active domain routes",
		Long:         "devcontainer-cli route ls — query the global Caddy router for all active domain routes.",
		Example:      "  devcontainer-cli route ls",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := service.RouterService{Report: console}
			_ = svc.ReconcileOrphans(cmd.Context())
			routes, err := svc.ListRoutes(cmd.Context())
			if err != nil {
				return err
			}

			if len(routes) == 0 {
				console.Info("No active domain routes found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "DOMAIN\tTARGET\tROUTE ID")
			for _, r := range routes {
				target := fmt.Sprintf("%s:%d", r.TargetHost, r.TargetPort)
				fmt.Fprintf(w, "%s\t%s\t%s\n", r.Domain, target, r.ID)
			}
			return w.Flush()
		},
	}
}

func newRouteRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "rm <id-or-domain>",
		Short:             "Remove a domain route from the router",
		Long:              "devcontainer-cli route rm — delete an active route by its Route ID or domain name.",
		Example:           "  devcontainer-cli route rm devcli_myws_1a2b3c4d\n  devcontainer-cli route rm api.devcli.localhost",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completeActiveRouteArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := strings.TrimSpace(args[0])
			svc := service.RouterService{Report: console}

			if err := svc.RemoveRoute(cmd.Context(), identifier); err != nil {
				return err
			}

			console.Success("✓ Route %q removed.", identifier)
			return nil
		},
	}
	return cmd
}

func completeActiveRouteArgs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	svc := service.RouterService{Report: console}
	routes, err := svc.ListRoutes(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for _, r := range routes {
		if strings.HasPrefix(r.Domain, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s:%d", r.Domain, r.TargetHost, r.TargetPort))
		}
		if strings.HasPrefix(r.ID, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s", r.ID, r.Domain))
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func newRouteStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Show status of the global Caddy router",
		Long:         "devcontainer-cli route status — inspect the health, ports, and connected networks of the router.",
		Example:      "  devcontainer-cli route status",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := service.RouterService{Report: console}
			st, err := svc.Status(cmd.Context())
			if err != nil {
				return err
			}

			if !st.Running {
				console.Warn("Global router container '%s' is not running.", st.ContainerName)
				return nil
			}

			console.Success("✓ Global router '%s' is running.", st.ContainerName)
			console.Info("  Admin URL:    %s", st.AdminURL)
			console.Info("  HTTP Port:    %d", st.HTTPPort)
			console.Info("  HTTPS Port:   %d", st.HTTPSPort)
			console.Info("  Active Routes:%d", st.ActiveRoutes)
			if len(st.Networks) > 0 {
				console.Info("  Networks:     %s", strings.Join(st.Networks, ", "))
			}
			return nil
		},
	}
}

func newRouteStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "stop",
		Short:        "Stop and remove the global Caddy router container",
		Long:         "devcontainer-cli route stop — stop and remove the devcli-router container.",
		Example:      "  devcontainer-cli route stop",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := service.RouterService{Report: console}
			return svc.StopRouter()
		},
	}
}
