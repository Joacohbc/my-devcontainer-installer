package commands

import (
	"errors"
	"testing"
)

func TestAccessInvocationsParseWithoutPositionalArgs(t *testing.T) {
	cases := []struct {
		choice   string
		wantFlag string
	}{
		{accessSSHEphemeral, "ephemeral"},
		{accessSSHSetup, "setup"},
		{accessShell, ""},
	}
	for _, tc := range cases {
		t.Run(tc.choice, func(t *testing.T) {
			inv, ok := accessInvocations[tc.choice]
			if !ok {
				t.Fatalf("no invocation for %q", tc.choice)
			}
			cmd := inv.newCommand()
			if err := cmd.ParseFlags(inv.flags); err != nil {
				t.Fatal(err)
			}
			if got := cmd.Flags().Args(); len(got) != 0 {
				t.Errorf("positional args = %v, want none: they would run inside the container", got)
			}
			if tc.wantFlag == "" {
				return
			}
			if v, _ := cmd.Flags().GetBool(tc.wantFlag); !v {
				t.Errorf("--%s not set", tc.wantFlag)
			}
		})
	}
	if _, ok := accessInvocations[accessNone]; ok {
		t.Error("none must not run a session")
	}
}

func TestAccessOptionsOfferEveryChoiceWithNoneLast(t *testing.T) {
	opts := accessOptions()
	want := []string{accessSSHEphemeral, accessSSHSetup, accessShell, accessNone}
	if len(opts) != len(want) {
		t.Fatalf("got %d options, want %d", len(opts), len(want))
	}
	for i, w := range want {
		if opts[i].Value != w {
			t.Errorf("option %d = %q, want %q", i, opts[i].Value, w)
		}
	}
}

type fakeNextSteps struct {
	upAnswer     bool
	upErr        error
	pickChoice   string
	accessErr    error
	pickCalled   bool
	upCalled     bool
	accessChoice string
}

func (f *fakeNextSteps) hooks() nextStepHooks {
	return nextStepHooks{
		confirmUp: func() (bool, error) { return f.upAnswer, nil },
		up:        func() error { f.upCalled = true; return f.upErr },
		pickAccess: func() (string, error) {
			f.pickCalled = true
			return f.pickChoice, nil
		},
		runAccess: func(choice string) error {
			f.accessChoice = choice
			return f.accessErr
		},
	}
}

func TestOfferUpAndAccess(t *testing.T) {
	t.Run("declined up skips the menu", func(t *testing.T) {
		f := &fakeNextSteps{upAnswer: false}
		if err := offerUpAndAccess(f.hooks()); err != nil {
			t.Fatal(err)
		}
		if f.upCalled || f.pickCalled {
			t.Errorf("up=%v pick=%v, want neither", f.upCalled, f.pickCalled)
		}
	})

	t.Run("failed up is an error and skips the menu", func(t *testing.T) {
		f := &fakeNextSteps{upAnswer: true, upErr: errors.New("boom")}
		if err := offerUpAndAccess(f.hooks()); err == nil {
			t.Fatal("a failed up must fail the command")
		}
		if f.pickCalled {
			t.Error("menu offered after a failed up")
		}
	})

	t.Run("none runs no session", func(t *testing.T) {
		f := &fakeNextSteps{upAnswer: true, pickChoice: accessNone}
		if err := offerUpAndAccess(f.hooks()); err != nil {
			t.Fatal(err)
		}
		if f.accessChoice != "" {
			t.Errorf("ran %q, want nothing", f.accessChoice)
		}
	})

	t.Run("chosen access runs after up", func(t *testing.T) {
		f := &fakeNextSteps{upAnswer: true, pickChoice: accessShell}
		if err := offerUpAndAccess(f.hooks()); err != nil {
			t.Fatal(err)
		}
		if !f.upCalled || f.accessChoice != accessShell {
			t.Errorf("up=%v access=%q, want up then shell", f.upCalled, f.accessChoice)
		}
	})

	t.Run("a failing session does not fail the init", func(t *testing.T) {
		f := &fakeNextSteps{upAnswer: true, pickChoice: accessSSHSetup, accessErr: errors.New("exit 1")}
		if err := offerUpAndAccess(f.hooks()); err != nil {
			t.Fatalf("generation already succeeded; got %v", err)
		}
	})
}
