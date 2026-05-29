package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestUIHelpers(t *testing.T) {
	// 1. Test Log
	outLog := captureStdout(func() { Log("test log") })
	if !strings.Contains(outLog, "==>") || !strings.Contains(outLog, "test log") {
		t.Errorf("Log output mismatch, got: %q", outLog)
	}

	// 2. Test Ok
	outOk := captureStdout(func() { Ok("test ok") })
	if !strings.Contains(outOk, "✓") || !strings.Contains(outOk, "test ok") {
		t.Errorf("Ok output mismatch, got: %q", outOk)
	}

	// 3. Test Warn
	outWarn := captureStdout(func() { Warn("test warn") })
	if !strings.Contains(outWarn, "!") || !strings.Contains(outWarn, "test warn") {
		t.Errorf("Warn output mismatch, got: %q", outWarn)
	}

	// 4. Test Success
	outSuccess := captureStdout(func() { Success("test success") })
	if !strings.Contains(outSuccess, "test success") {
		t.Errorf("Success output mismatch, got: %q", outSuccess)
	}

	// 5. Test Bar
	outBar := captureStdout(func() { Bar() })
	if len(strings.TrimSpace(outBar)) == 0 {
		t.Error("Bar output was empty")
	}

	// 6. Test Green, Yellow, Red, Cyan
	outGreen := captureStdout(func() { Green("green %s", "val") })
	if !strings.Contains(outGreen, "green val") {
		t.Errorf("Green output mismatch, got: %q", outGreen)
	}

	outYellow := captureStdout(func() { Yellow("yellow %s", "val") })
	if !strings.Contains(outYellow, "yellow val") {
		t.Errorf("Yellow output mismatch, got: %q", outYellow)
	}

	outRed := captureStdout(func() { Red("red %s", "val") })
	if !strings.Contains(outRed, "red val") {
		t.Errorf("Red output mismatch, got: %q", outRed)
	}

	outCyan := captureStdout(func() { Cyan("cyan %s", "val") })
	if !strings.Contains(outCyan, "cyan val") {
		t.Errorf("Cyan output mismatch, got: %q", outCyan)
	}

	// 7. Test Sprint variants
	sGreen := GreenS("green %s", "val")
	if !strings.Contains(sGreen, "green val") {
		t.Errorf("GreenS mismatch, got: %q", sGreen)
	}

	sYellow := YellowS("yellow %s", "val")
	if !strings.Contains(sYellow, "yellow val") {
		t.Errorf("YellowS mismatch, got: %q", sYellow)
	}

	sRed := RedS("red %s", "val")
	if !strings.Contains(sRed, "red val") {
		t.Errorf("RedS mismatch, got: %q", sRed)
	}

	sCyan := CyanS("cyan %s", "val")
	if !strings.Contains(sCyan, "cyan val") {
		t.Errorf("CyanS mismatch, got: %q", sCyan)
	}

	// 8. Test Inline Styling
	sBold := Bold("bold text")
	if !strings.Contains(sBold, "bold text") {
		t.Errorf("Bold mismatch, got: %q", sBold)
	}

	sSubtle := Subtle("subtle text")
	if !strings.Contains(sSubtle, "subtle text") {
		t.Errorf("Subtle mismatch, got: %q", sSubtle)
	}

	// 9. Test Header
	outHeader := captureStdout(func() { Header("header text") })
	if !strings.Contains(outHeader, "header text") {
		t.Errorf("Header output mismatch, got: %q", outHeader)
	}

	// 10. Test Done and Cancelled
	outDone := captureStdout(func() { Done() })
	if !strings.Contains(outDone, "Done.") {
		t.Errorf("Done output mismatch, got: %q", outDone)
	}

	outCancelled := captureStdout(func() { Cancelled() })
	if !strings.Contains(outCancelled, "Cancelled.") {
		t.Errorf("Cancelled output mismatch, got: %q", outCancelled)
	}
}
