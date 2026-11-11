package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	sv "github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// A select field's Validate is enforced by the rendered form: a refused choice
// is reported and asked again, so service-level checks (such as the wizard
// refusing an unbuilt image) reach the user.
func TestBuildStepFormSelectAppliesValidate(t *testing.T) {
	field := sv.Field{
		Kind:    sv.FieldSelect,
		Title:   "Image:",
		Choices: []sv.Option{{Value: "unbuilt", Label: "unbuilt"}, {Value: "built", Label: "built"}},
		Initial: "unbuilt",
		Validate: func(v string) error {
			if v == "unbuilt" {
				return errors.New("image is not built")
			}
			return nil
		},
	}
	value, form := buildStepForm(field)

	var out bytes.Buffer
	if err := form.WithAccessible(true).WithInput(&lineReader{lines: []string{"1\n", "2\n"}}).WithOutput(&out).Run(); err != nil {
		t.Fatalf("form: %v", err)
	}
	if got := value(); got != "built" {
		t.Errorf("value = %v, want the refused choice asked again", got)
	}
	if !strings.Contains(out.String(), "image is not built") {
		t.Errorf("output %q lacks the validation error", out.String())
	}
}

// lineReader hands out one line per Read, the way a terminal does, so each
// accessible prompt reads only its own answer.
type lineReader struct {
	lines []string
}

func (r *lineReader) Read(p []byte) (int, error) {
	if len(r.lines) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.lines[0])
	r.lines = r.lines[1:]
	return n, nil
}
