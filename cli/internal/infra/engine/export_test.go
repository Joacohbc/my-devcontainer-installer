package engine

import "io"

// CaptureComposeHint redirects the Podman compose hint for the duration of a test.
func CaptureComposeHint(w io.Writer) (restore func()) {
	previous := composeHintOutput
	composeHintOutput = w
	return func() { composeHintOutput = previous }
}
