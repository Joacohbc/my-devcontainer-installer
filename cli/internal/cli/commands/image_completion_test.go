package commands

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/spf13/cobra"
)

func seedImageDefs(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, def := range []domain.ImageDef{
		{Name: "android", Source: domain.ImageSourceBuild},
		{Name: "api", Source: domain.ImageSourceBuild, Label: "API toolchain"},
		{Name: "team-base", Source: domain.ImageSourceRemote, Ref: "ghcr.io/acme/base:1"},
	} {
		if err := domain.SaveImageDef(def, domain.UserImageDir()); err != nil {
			t.Fatalf("SaveImageDef(%s): %v", def.Name, err)
		}
	}
}

func imageSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	sub, _, err := newImageCommand().Find([]string{name})
	if err != nil || sub.Name() != name {
		t.Fatalf("image %s not found: %v", name, err)
	}
	return sub
}

func TestImageSubcommandsCompleteImageNames(t *testing.T) {
	seedImageDefs(t)

	cases := []struct {
		sub        string
		args       []string
		toComplete string
		want       []string
		directive  cobra.ShellCompDirective
	}{
		{"info", nil, "", []string{"android", "api", "team-base"}, cobra.ShellCompDirectiveNoFileComp},
		{"info", nil, "a", []string{"android", "api"}, cobra.ShellCompDirectiveNoFileComp},
		{"info", []string{"api"}, "", nil, cobra.ShellCompDirectiveNoFileComp},
		{"remove", nil, "t", []string{"team-base"}, cobra.ShellCompDirectiveNoFileComp},
		{"build", nil, "", []string{"android", "api"}, cobra.ShellCompDirectiveNoFileComp},
		{"pull", nil, "", []string{"team-base"}, cobra.ShellCompDirectiveNoFileComp},
		{"export", nil, "an", []string{"android"}, cobra.ShellCompDirectiveNoFileComp},
		{"export", []string{"android"}, "", nil, cobra.ShellCompDirectiveFilterDirs},
		{"export", []string{"android", "out"}, "", nil, cobra.ShellCompDirectiveNoFileComp},
	}
	for _, c := range cases {
		t.Run(c.sub+"/"+strings.Join(c.args, ",")+"/"+c.toComplete, func(t *testing.T) {
			cmd := imageSubcommand(t, c.sub)
			if cmd.ValidArgsFunction == nil {
				t.Fatalf("image %s has no ValidArgsFunction", c.sub)
			}
			got, directive := cmd.ValidArgsFunction(cmd, c.args, c.toComplete)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("completions = %v, want %v", got, c.want)
			}
			if directive != c.directive {
				t.Errorf("directive = %v, want %v", directive, c.directive)
			}
		})
	}
}

func TestImageCreateFlagsComplete(t *testing.T) {
	seedImageDefs(t)
	cmd := imageSubcommand(t, "create")

	cases := []struct {
		flag       string
		toComplete string
		contains   string
		excludes   string
	}{
		{"from", "", "team-base", ""},
		{"from", "an", "android", "api"},
		{"from-profile", "", "nodejs", ""},
		{flagModules, "nodejs,", "nodejs,go", "nodejs,nodejs"},
	}
	for _, c := range cases {
		t.Run(c.flag+"/"+c.toComplete, func(t *testing.T) {
			fn, ok := cmd.GetFlagCompletionFunc(c.flag)
			if !ok {
				t.Fatalf("image create --%s has no completion", c.flag)
			}
			got, _ := fn(cmd, nil, c.toComplete)
			joined := "," + strings.Join(got, ",") + ","
			if !strings.Contains(joined, ","+c.contains+",") {
				t.Errorf("--%s %q completions %v lack %q", c.flag, c.toComplete, got, c.contains)
			}
			if c.excludes != "" && strings.Contains(joined, ","+c.excludes+",") {
				t.Errorf("--%s %q completions %v should not offer %q", c.flag, c.toComplete, got, c.excludes)
			}
		})
	}
}

func TestImageFlagCompletesImageNames(t *testing.T) {
	seedImageDefs(t)

	for _, cmd := range []*cobra.Command{NewRootCommand("test"), newInitCommand(), newAgentCreateCommand()} {
		fn, ok := cmd.GetFlagCompletionFunc(flagImage)
		if !ok {
			t.Fatalf("%s --image has no completion", cmd.Name())
		}
		got, directive := fn(cmd, nil, "a")
		if strings.Join(got, ",") != "android,api" {
			t.Errorf("%s --image a = %v, want [android api]", cmd.Name(), got)
		}
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("%s --image directive = %v, want NoFileComp", cmd.Name(), directive)
		}
	}
}
