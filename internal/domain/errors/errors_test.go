package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestExitCodeByKind(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "validation", err: New(KindValidation, "bad input", nil), expected: 2},
		{name: "auth", err: New(KindAuthentication, "no token", nil), expected: 3},
		{name: "not found", err: New(KindNotFound, "missing", nil), expected: 4},
		{name: "conflict", err: New(KindConflict, "exists", nil), expected: 5},
		{name: "transient", err: New(KindTransient, "timeout", nil), expected: 10},
		{name: "not implemented", err: New(KindNotImplemented, "todo", nil), expected: 11},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := ExitCode(test.err); actual != test.expected {
				t.Fatalf("expected %d, got %d", test.expected, actual)
			}
		})
	}
}

func TestErrorFormattingAndUnwrap(t *testing.T) {
	withoutCause := New(KindValidation, "bad input", nil)
	if !strings.Contains(withoutCause.Error(), "validation: bad input") {
		t.Fatalf("unexpected error string without cause: %q", withoutCause.Error())
	}

	cause := errors.New("boom")
	withCause := New(KindTransient, "request failed", cause)
	if !strings.Contains(withCause.Error(), "transient: request failed") || !strings.Contains(withCause.Error(), "boom") {
		t.Fatalf("unexpected error string with cause: %q", withCause.Error())
	}
	if !errors.Is(withCause, cause) {
		t.Fatal("expected unwrap to expose cause")
	}
}

func TestExitCodeDefaults(t *testing.T) {
	if ExitCode(nil) != 0 {
		t.Fatal("expected nil error exit code 0")
	}

	if ExitCode(errors.New("plain")) != 1 {
		t.Fatal("expected plain error exit code 1")
	}

	if ExitCode(New(KindInternal, "internal", nil)) != 1 {
		t.Fatal("expected internal app error exit code 1")
	}
	if ExitCode(New(KindPermanent, "permanent", nil)) != 1 {
		t.Fatal("expected permanent app error exit code 1")
	}
	if ExitCode(New(KindAuthorization, "forbidden", nil)) != 3 {
		t.Fatal("expected authorization exit code 3")
	}
}

func TestKindOf(t *testing.T) {
	if got := KindOf(nil); got != "" {
		t.Fatalf("expected empty kind for nil error, got %q", got)
	}

	if got := KindOf(New(KindValidation, "bad", nil)); got != KindValidation {
		t.Fatalf("expected validation kind, got %q", got)
	}

	if got := KindOf(errors.New("plain")); got != KindInternal {
		t.Fatalf("expected internal kind for plain error, got %q", got)
	}
}
