package miro

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunWithRecoverPassesThroughNilError(t *testing.T) {
	err := RunWithRecover(nil, func() error { return nil })
	if err != nil {
		t.Errorf("RunWithRecover(nil-fn) = %v, want nil", err)
	}
}

func TestRunWithRecoverPassesThroughError(t *testing.T) {
	want := errors.New("planned failure")
	got := RunWithRecover(nil, func() error { return want })
	if !errors.Is(got, want) {
		t.Errorf("RunWithRecover did not pass through error: %v", got)
	}
}

func TestRunWithRecoverConvertsPanicToPanicError(t *testing.T) {
	err := RunWithRecover(nil, func() error { panic("boom") })
	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("err type = %T, want *PanicError", err)
	}
	if pe.CorrelationID == "" {
		t.Error("PanicError missing CorrelationID")
	}
	if strings.Contains(pe.Error(), "boom") {
		t.Errorf("PanicError.Error leaked panic value: %q", pe.Error())
	}
	if !strings.Contains(pe.Error(), pe.CorrelationID) {
		t.Errorf("PanicError.Error missing id: %q", pe.Error())
	}
}

func TestRunWithRecoverSuppressesStackByDefault(t *testing.T) {
	t.Setenv(EnvDebug, "")
	var buf bytes.Buffer
	_ = RunWithRecover(&buf, func() error { panic("boom") })
	if buf.Len() != 0 {
		t.Errorf("expected no diagnostics without %s, got %q", EnvDebug, buf.String())
	}
}

func TestRunWithRecoverEmitsStackInDebug(t *testing.T) {
	t.Setenv(EnvDebug, "1")
	var buf bytes.Buffer
	_ = RunWithRecover(&buf, func() error { panic("boom-debug") })
	if !strings.Contains(buf.String(), "boom-debug") {
		t.Errorf("expected diagnostics under %s=1 to include panic value, got %q", EnvDebug, buf.String())
	}
	if !strings.Contains(buf.String(), "miro: panic") {
		t.Errorf("expected diagnostics to be tagged 'miro: panic', got %q", buf.String())
	}
}

func TestPanicErrorIsClassifiedAsAPIExit(t *testing.T) {
	// PanicError is not a *ConfigError or *APIError, so it must map to
	// ExitAPI via the default branch. This keeps scripts that switch on
	// $? predictable when the CLI crashes.
	err := &PanicError{CorrelationID: "abcd1234"}
	if got := ExitCode(err); got != ExitAPI {
		t.Errorf("ExitCode(*PanicError) = %d, want %d", got, ExitAPI)
	}
}
