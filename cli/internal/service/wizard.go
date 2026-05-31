package service

// FieldKind selects which input widget a wizard step renders.
type FieldKind int

const (
	FieldSelect FieldKind = iota
	FieldMultiselect
	FieldInput
	FieldConfirm
)

// Field is the declarative description of a single wizard screen. A step's
// Build seeds Initial from prior answers so revisiting a step (via esc-back)
// shows the value the user last entered.
type Field struct {
	Kind     FieldKind
	Title    string
	Choices  []Option
	Initial  any
	Validate func(string) error
}

// Step is one wizard screen. Key identifies the answer in State; Build produces
// the field to render given the answers gathered so far.
type Step struct {
	Key   string
	Build func(s *State) Field
}

// State carries the answers collected so far. Keys are step Keys; values are
// typed per FieldKind (string / []string / bool). It is passed to every Build
// so steps can both seed themselves and branch on earlier answers.
type State struct {
	values map[string]any
}

// NewState returns an empty wizard State.
func NewState() *State { return &State{values: map[string]any{}} }

// Set records the answer for key. The wizard engine calls this as each step completes.
func (s *State) Set(key string, value any) { s.values[key] = value }

// Has reports whether an answer was recorded for key.
func (s *State) Has(key string) bool { _, ok := s.values[key]; return ok }

// String returns the string answer for key (empty if absent/wrong type).
func (s *State) String(key string) string { v, _ := s.values[key].(string); return v }

// Strings returns the []string answer for key (nil if absent/wrong type).
func (s *State) Strings(key string) []string { v, _ := s.values[key].([]string); return v }

// Bool returns the bool answer for key (false if absent/wrong type).
func (s *State) Bool(key string) bool { v, _ := s.values[key].(bool); return v }
