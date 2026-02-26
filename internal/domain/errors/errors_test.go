package errors

import "testing"

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
