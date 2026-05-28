package commands

import (
	"errors"
	"testing"
)

func TestResolveRemoveVolumes(t *testing.T) {
	confirmErr := errors.New("boom")
	cases := []struct {
		name        string
		volumesFlag bool
		yes         bool
		interactive bool
		confirm     func() (bool, error)
		want        bool
		wantErr     bool
		wantConfirm bool
	}{
		{name: "explicit -v removes volumes", volumesFlag: true, want: true},
		{name: "-v wins even with --yes", volumesFlag: true, yes: true, interactive: true, want: true},
		{name: "--yes only skips prompt, keeps volumes", yes: true, interactive: true, want: false},
		{name: "--yes non-interactive keeps volumes", yes: true, interactive: false, want: false},
		{name: "non-interactive without flags keeps volumes", interactive: false, want: false},
		{name: "interactive confirm yes", interactive: true, confirm: func() (bool, error) { return true, nil }, want: true, wantConfirm: true},
		{name: "interactive confirm no", interactive: true, confirm: func() (bool, error) { return false, nil }, want: false, wantConfirm: true},
		{name: "interactive confirm error propagates", interactive: true, confirm: func() (bool, error) { return false, confirmErr }, wantErr: true, wantConfirm: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			confirm := func() (bool, error) {
				called = true
				if tc.confirm != nil {
					return tc.confirm()
				}
				t.Fatal("confirm should not have been called")
				return false, nil
			}
			got, err := resolveRemoveVolumes(tc.volumesFlag, tc.yes, tc.interactive, confirm)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveRemoveVolumes = %v, want %v", got, tc.want)
			}
			if called != tc.wantConfirm {
				t.Errorf("confirm called = %v, want %v", called, tc.wantConfirm)
			}
		})
	}
}
