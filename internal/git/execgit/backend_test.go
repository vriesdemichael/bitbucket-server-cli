package execgit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
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

func TestFetchAndCheckoutValidation(t *testing.T) {
	backend := New()
	backend.Timeout = time.Second

	err := backend.Fetch(context.Background(), "", git.FetchOptions{})
	if err == nil {
		t.Fatal("expected validation error for empty repository directory")
	}

	err = backend.Checkout(context.Background(), "", git.CheckoutOptions{Ref: "main"})
	if err == nil {
		t.Fatal("expected validation error for empty checkout directory")
	}

	err = backend.Checkout(context.Background(), ".", git.CheckoutOptions{Ref: ""})
	if err == nil {
		t.Fatal("expected validation error for empty checkout ref")
	}
}

func TestRunValidationAndFailure(t *testing.T) {
	backend := New()
	backend.Timeout = time.Second

	_, err := backend.run(context.Background(), runOptions{})
	if err == nil {
		t.Fatal("expected validation error for empty git args")
	}
	if apperrors.ExitCode(err) != 2 {
		t.Fatalf("expected validation exit code 2, got %d", apperrors.ExitCode(err))
	}

	_, err = backend.run(context.Background(), runOptions{args: []string{"definitely-not-a-git-command"}})
	if err == nil {
		t.Fatal("expected permanent git command failure")
	}
	if apperrors.ExitCode(err) != 1 {
		t.Fatalf("expected permanent exit code 1, got %d", apperrors.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "git definitely-not-a-git-command failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCloneAndCheckoutAgainstLocalRepo(t *testing.T) {
	backend := New()
	backend.Timeout = 5 * time.Second

	temporary := t.TempDir()
	remoteDir := filepath.Join(temporary, "remote.git")
	workDir := filepath.Join(temporary, "work")
	cloneDir := filepath.Join(temporary, "clone")

	if _, err := backend.run(context.Background(), runOptions{args: []string{"init", "--bare", remoteDir}}); err != nil {
		t.Fatalf("failed to initialize bare repository: %v", err)
	}

	if _, err := backend.run(context.Background(), runOptions{args: []string{"init", workDir}}); err != nil {
		t.Fatalf("failed to initialize working repository: %v", err)
	}

	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"config", "user.email", "test@example.local"}}); err != nil {
		t.Fatalf("failed to configure user.email: %v", err)
	}
	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"config", "user.name", "Test User"}}); err != nil {
		t.Fatalf("failed to configure user.name: %v", err)
	}

	if writeErr := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("seed\n"), 0o644); writeErr != nil {
		t.Fatalf("failed to write seed file: %v", writeErr)
	}

	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"add", "README.md"}}); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"commit", "-m", "seed"}}); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}
	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"branch", "-M", "main"}}); err != nil {
		t.Fatalf("failed to rename branch to main: %v", err)
	}
	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"remote", "add", "origin", remoteDir}}); err != nil {
		t.Fatalf("failed to add origin remote: %v", err)
	}
	if _, err := backend.run(context.Background(), runOptions{cwd: workDir, args: []string{"push", "-u", "origin", "main"}}); err != nil {
		t.Fatalf("failed to push main branch: %v", err)
	}

	if err := backend.Clone(context.Background(), remoteDir, git.CloneOptions{Directory: cloneDir, Branch: "main", Depth: 1}); err != nil {
		t.Fatalf("expected clone to succeed, got: %v", err)
	}

	if err := backend.Fetch(context.Background(), cloneDir, git.FetchOptions{Remote: "origin"}); err != nil {
		t.Fatalf("expected fetch to succeed, got: %v", err)
	}

	if err := backend.Checkout(context.Background(), cloneDir, git.CheckoutOptions{Ref: "main"}); err != nil {
		t.Fatalf("expected checkout to succeed, got: %v", err)
	}
}
