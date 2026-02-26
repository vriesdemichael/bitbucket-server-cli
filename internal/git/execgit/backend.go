package execgit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/git"
)

const defaultTimeout = 60 * time.Second

type Backend struct {
	Timeout time.Duration
}

func New() *Backend {
	return &Backend{Timeout: defaultTimeout}
}

func (backend *Backend) Version(ctx context.Context) (string, error) {
	result, err := backend.run(ctx, runOptions{args: []string{"--version"}})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result.stdout), nil
}

func (backend *Backend) Clone(ctx context.Context, repositoryURL string, options git.CloneOptions) error {
	if strings.TrimSpace(repositoryURL) == "" {
		return apperrors.New(apperrors.KindValidation, "repository URL cannot be empty", nil)
	}

	if strings.TrimSpace(options.Directory) == "" {
		return apperrors.New(apperrors.KindValidation, "clone directory cannot be empty", nil)
	}

	args := []string{"clone", repositoryURL, options.Directory}
	if options.Branch != "" {
		args = append(args, "--branch", options.Branch)
	}
	if options.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", options.Depth))
	}

	_, err := backend.run(ctx, runOptions{args: args})
	return err
}

func (backend *Backend) Fetch(ctx context.Context, repositoryDirectory string, options git.FetchOptions) error {
	if strings.TrimSpace(repositoryDirectory) == "" {
		return apperrors.New(apperrors.KindValidation, "repository directory cannot be empty", nil)
	}

	args := []string{"fetch"}
	if strings.TrimSpace(options.Remote) != "" {
		args = append(args, options.Remote)
	}

	_, err := backend.run(ctx, runOptions{cwd: repositoryDirectory, args: args})
	return err
}

func (backend *Backend) Checkout(ctx context.Context, repositoryDirectory string, options git.CheckoutOptions) error {
	if strings.TrimSpace(repositoryDirectory) == "" {
		return apperrors.New(apperrors.KindValidation, "repository directory cannot be empty", nil)
	}

	if strings.TrimSpace(options.Ref) == "" {
		return apperrors.New(apperrors.KindValidation, "checkout ref cannot be empty", nil)
	}

	_, err := backend.run(ctx, runOptions{cwd: repositoryDirectory, args: []string{"checkout", options.Ref}})
	return err
}

type runOptions struct {
	cwd  string
	args []string
}

type runResult struct {
	stdout string
	stderr string
}

func (backend *Backend) run(ctx context.Context, options runOptions) (runResult, error) {
	if len(options.args) == 0 {
		return runResult{}, apperrors.New(apperrors.KindValidation, "git command cannot be empty", nil)
	}

	if backend.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, backend.Timeout)
		defer cancel()
	}

	command := exec.CommandContext(ctx, "git", options.args...)
	if options.cwd != "" {
		command.Dir = options.cwd
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := runResult{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		message := strings.TrimSpace(result.stderr)
		if message == "" {
			message = strings.TrimSpace(err.Error())
		}
		return result, apperrors.New(apperrors.KindPermanent, fmt.Sprintf("git %s failed: %s", strings.Join(options.args, " "), message), err)
	}

	return result, nil
}
