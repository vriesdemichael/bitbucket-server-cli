package execgit

import (
	"context"
	"testing"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/git"
)

func TestCloneValidation(t *testing.T) {
	backend := New()
	backend.Timeout = time.Second

	err := backend.Clone(context.Background(), "", git.CloneOptions{Directory: "tmp"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVersion(t *testing.T) {
	backend := New()
	backend.Timeout = 5 * time.Second

	version, err := backend.Version(context.Background())
	if err != nil {
		t.Fatalf("expected git version, got error: %v", err)
	}

	if version == "" {
		t.Fatal("expected non-empty git version")
	}
}
