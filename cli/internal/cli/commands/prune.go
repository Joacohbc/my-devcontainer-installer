package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func init() { register(newPruneCommand()) }

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove orphan devcontainer-cli/* images",
		Long: `devcontainer-cli prune — remove devcontainer-cli images whose projects no longer exist

By default removes only orphan images (project directory is gone).
Use --all to remove every devcontainer-cli/* image regardless.`,
		SilenceUsage: true,
		RunE:         runPrune,
	}
	cmd.Flags().Bool("all", false, "Remove ALL devcontainer-cli/* images, not just orphans")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

type localImage struct {
	ref string
	id  string
}

func listCliImages() []localImage {
	status, stdout, _, err := docker.DockerCapture([]string{
		"images", "--filter", "reference=" + types.ImageNamespace + "/*",
		"--format", "{{.Repository}}:{{.Tag}}\t{{.ID}}",
	})
	if err != nil || status != 0 || strings.TrimSpace(stdout) == "" {
		return nil
	}
	var out []localImage
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, localImage{ref: strings.TrimSpace(parts[0]), id: strings.TrimSpace(parts[1])})
	}
	return out
}

func runPrune(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")

	localImages := listCliImages()
	if len(localImages) == 0 {
		color.New(color.FgWhite).Println("No devcontainer-cli/* images found locally.")
		return nil
	}

	var toRemove []localImage
	if all {
		toRemove = localImages
	} else {
		entries := domain.ListEntries()
		tracked := make(map[string]bool)
		live := make(map[string]bool)
		for _, e := range entries {
			tracked[e.Image] = true
			if _, err := os.Stat(e.ProjectDir); err == nil {
				live[e.Image] = true
			}
		}
		for _, img := range localImages {
			if !tracked[img.ref] || !live[img.ref] {
				toRemove = append(toRemove, img)
			}
		}
	}

	if len(toRemove) == 0 {
		gray := color.New(color.FgWhite)
		gray.Println("No orphan devcontainer-cli/* images found.")
		gray.Println("Use --all to remove every devcontainer-cli/* image.")
		return nil
	}

	color.Yellow("\nImages to remove (%d):", len(toRemove))
	for _, img := range toRemove {
		color.New(color.FgWhite).Printf("  %s  (%s)\n", img.ref, img.id)
	}
	println()

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("cannot prune images in non-interactive mode without --yes")
		}
		proceed, err := prompt.Confirm("Remove these images?", false)
		if err != nil {
			return err
		}
		if !proceed {
			color.Yellow("Cancelled.")
			return nil
		}
	}

	okCount, failCount := 0, 0
	for _, img := range toRemove {
		status, _ := docker.DockerInherit([]string{"rmi", img.ref})
		if status == 0 {
			okCount++
		} else {
			color.Red("  ✗ Failed to remove %s (container may be running)", img.ref)
			failCount++
		}
	}

	msg := color.New(color.FgGreen, color.Bold).Sprintf("\nRemoved %d image(s).", okCount)
	if failCount > 0 {
		msg += color.RedString(" %d failed.", failCount)
	}
	println(msg)
	return nil
}
