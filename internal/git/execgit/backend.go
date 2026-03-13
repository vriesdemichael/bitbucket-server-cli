package execgit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
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
	if len(options.ExtraArgs) > 0 {
		args = append(args, options.ExtraArgs...)
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

func (backend *Backend) AddRemote(ctx context.Context, repositoryDirectory string, remote git.Remote) error {
	if strings.TrimSpace(repositoryDirectory) == "" {
		return apperrors.New(apperrors.KindValidation, "repository directory cannot be empty", nil)
	}

	name := strings.TrimSpace(remote.Name)
	if name == "" {
		return apperrors.New(apperrors.KindValidation, "remote name cannot be empty", nil)
	}

	remoteURL := strings.TrimSpace(remote.URL)
	if remoteURL == "" {
		return apperrors.New(apperrors.KindValidation, "remote URL cannot be empty", nil)
	}

	_, err := backend.run(ctx, runOptions{cwd: repositoryDirectory, args: []string{"remote", "add", name, remoteURL}})
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

func (backend *Backend) RepositoryRoot(ctx context.Context, workingDirectory string) (string, error) {
	result, err := backend.run(ctx, runOptions{cwd: strings.TrimSpace(workingDirectory), args: []string{"rev-parse", "--show-toplevel"}})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result.stdout), nil
}

func (backend *Backend) ListRemotes(ctx context.Context, repositoryDirectory string) ([]git.Remote, error) {
	trimmedDir := strings.TrimSpace(repositoryDirectory)
	if trimmedDir == "" {
		return nil, apperrors.New(apperrors.KindValidation, "repository directory cannot be empty", nil)
	}

	result, err := backend.run(ctx, runOptions{cwd: trimmedDir, args: []string{"remote", "-v"}})
	if err != nil {
		return nil, err
	}

	lines := strings.Split(result.stdout, "\n")
	seen := map[string]struct{}{}
	remotes := make([]git.Remote, 0)
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if fields[2] != "(fetch)" {
			continue
		}

		name := strings.TrimSpace(fields[0])
		url := strings.TrimSpace(fields[1])
		if name == "" || url == "" {
			continue
		}

		key := name + "\x00" + url
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		remotes = append(remotes, git.Remote{Name: name, URL: url})
	}

	sort.SliceStable(remotes, func(left, right int) bool {
		if remotes[left].Name == remotes[right].Name {
			return remotes[left].URL < remotes[right].URL
		}
		if remotes[left].Name == "origin" {
			return true
		}
		if remotes[right].Name == "origin" {
			return false
		}
		return remotes[left].Name < remotes[right].Name
	})

	return remotes, nil
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
