//go:build live

package live_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type seededProject struct {
	Key   string
	Repos []seededRepository
}

type seededRepository struct {
	Name      string
	Slug      string
	CommitIDs []string
}

type liveHarness struct {
	t      *testing.T
	config config.AppConfig
	client *openapigenerated.ClientWithResponses
}

func newLiveHarness(t *testing.T) *liveHarness {
	t.Helper()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.BitbucketUsername == "" || cfg.BitbucketPassword == "" {
		t.Skip("BITBUCKET_USERNAME/BITBUCKET_PASSWORD (or ADMIN_USER/ADMIN_PASSWORD) required for live harness")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable is required for commit seeding")
	}

	client, err := newGeneratedClient(cfg)
	if err != nil {
		t.Fatalf("create generated client: %v", err)
	}

	return &liveHarness{t: t, config: cfg, client: client}
}

func (h *liveHarness) seedProjectWithRepositories(ctx context.Context, repositoryCount int, commitsPerRepository int) (seededProject, error) {
	if repositoryCount < 1 {
		return seededProject{}, fmt.Errorf("repository count must be >= 1")
	}
	if commitsPerRepository < 1 {
		return seededProject{}, fmt.Errorf("commits per repository must be >= 1")
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	projectKey := strings.ToUpper("LT" + suffix[len(suffix)-6:])
	projectName := "Live Test " + suffix

	createProjectBody := openapigenerated.CreateProjectJSONRequestBody{Key: &projectKey, Name: &projectName}
	createProjectResponse, err := h.client.CreateProjectWithResponse(ctx, createProjectBody)
	if err != nil {
		return seededProject{}, fmt.Errorf("create project call failed: %w", err)
	}
	if createProjectResponse.StatusCode() < 200 || createProjectResponse.StatusCode() >= 300 {
		return seededProject{}, fmt.Errorf("create project returned status %d", createProjectResponse.StatusCode())
	}

	seeded := seededProject{Key: projectKey, Repos: make([]seededRepository, 0, repositoryCount)}
	h.t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = h.cleanupProject(cleanupCtx, seeded)
	})

	for index := 0; index < repositoryCount; index++ {
		repoName := fmt.Sprintf("lt-repo-%d-%s", index+1, suffix[len(suffix)-4:])
		scmID := "git"
		forkable := true
		createRepoBody := openapigenerated.CreateRepositoryJSONRequestBody{Name: &repoName, ScmId: &scmID, Forkable: &forkable}
		createRepoResponse, createErr := h.client.CreateRepositoryWithResponse(ctx, projectKey, createRepoBody)
		if createErr != nil {
			return seededProject{}, fmt.Errorf("create repository call failed: %w", createErr)
		}
		if createRepoResponse.StatusCode() < 200 || createRepoResponse.StatusCode() >= 300 {
			return seededProject{}, fmt.Errorf("create repository returned status %d", createRepoResponse.StatusCode())
		}

		repoSlug := repoName
		if createRepoResponse.ApplicationjsonCharsetUTF8201 != nil && createRepoResponse.ApplicationjsonCharsetUTF8201.Slug != nil {
			repoSlug = *createRepoResponse.ApplicationjsonCharsetUTF8201.Slug
		}

		if err := h.pushCommitsToRepository(projectKey, repoSlug, commitsPerRepository); err != nil {
			return seededProject{}, err
		}

		commitIDs, err := h.listCommitIDs(ctx, projectKey, repoSlug, commitsPerRepository+2)
		if err != nil {
			return seededProject{}, err
		}

		seeded.Repos = append(seeded.Repos, seededRepository{Name: repoName, Slug: repoSlug, CommitIDs: commitIDs})
	}

	return seeded, nil
}

func (h *liveHarness) pushCommitsToRepository(projectKey, repositorySlug string, commitCount int) error {
	tempDir := h.t.TempDir()

	if err := runGit(tempDir, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if err := runGit(tempDir, "checkout", "-b", "master"); err != nil {
		return fmt.Errorf("git checkout master failed: %w", err)
	}
	if err := runGit(tempDir, "config", "user.name", "bbsc-live-test"); err != nil {
		return fmt.Errorf("git config user.name failed: %w", err)
	}
	if err := runGit(tempDir, "config", "user.email", "bbsc-live-test@example.local"); err != nil {
		return fmt.Errorf("git config user.email failed: %w", err)
	}

	for index := 0; index < commitCount; index++ {
		filePath := filepath.Join(tempDir, "seed.txt")
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open seed file: %w", err)
		}
		_, _ = file.WriteString(fmt.Sprintf("commit-%d\n", index+1))
		_ = file.Close()

		if err := runGit(tempDir, "add", "seed.txt"); err != nil {
			return fmt.Errorf("git add failed: %w", err)
		}
		if err := runGit(tempDir, "commit", "-m", fmt.Sprintf("seed commit %d", index+1)); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}
	}

	pushURL, err := repositoryPushURL(h.config, projectKey, repositorySlug)
	if err != nil {
		return err
	}

	if err := runGit(tempDir, "remote", "add", "origin", pushURL); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}
	if err := runGit(tempDir, "push", "-u", "origin", "master"); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

func (h *liveHarness) pushCommitOnBranch(projectKey, repositorySlug, branch, fileName string) error {
	tempDir := h.t.TempDir()

	if err := runGit(tempDir, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if err := runGit(tempDir, "config", "user.name", "bbsc-live-test"); err != nil {
		return fmt.Errorf("git config user.name failed: %w", err)
	}
	if err := runGit(tempDir, "config", "user.email", "bbsc-live-test@example.local"); err != nil {
		return fmt.Errorf("git config user.email failed: %w", err)
	}

	pushURL, err := repositoryPushURL(h.config, projectKey, repositorySlug)
	if err != nil {
		return err
	}

	if err := runGit(tempDir, "remote", "add", "origin", pushURL); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}
	if err := runGit(tempDir, "fetch", "origin", "master"); err != nil {
		return fmt.Errorf("git fetch origin master failed: %w", err)
	}
	if err := runGit(tempDir, "checkout", "-b", branch, "FETCH_HEAD"); err != nil {
		return fmt.Errorf("git checkout branch failed: %w", err)
	}

	filePath := filepath.Join(tempDir, fileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open branch file: %w", err)
	}
	_, _ = file.WriteString(fmt.Sprintf("branch=%s\n", branch))
	_ = file.Close()

	if err := runGit(tempDir, "add", fileName); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	if err := runGit(tempDir, "commit", "-m", "seed branch commit"); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	if err := runGit(tempDir, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("git push branch failed: %w", err)
	}

	return nil
}

func (h *liveHarness) createPullRequest(ctx context.Context, projectKey, repositorySlug, fromBranch, toBranch string) (string, error) {
	type ref struct {
		Id string `json:"id"`
	}
	type body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		FromRef     ref    `json:"fromRef"`
		ToRef       ref    `json:"toRef"`
	}

	payload := body{
		Title:       "Live test PR",
		Description: "PR seeded by live harness",
		FromRef:     ref{Id: "refs/heads/" + fromBranch},
		ToRef:       ref{Id: "refs/heads/" + toBranch},
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal pull request payload: %w", err)
	}

	endpoint := strings.TrimRight(h.config.BitbucketURL, "/") + "/rest/api/latest/projects/" + projectKey + "/repos/" + repositorySlug + "/pull-requests"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawPayload))
	if err != nil {
		return "", fmt.Errorf("build pull request request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	if h.config.BitbucketToken != "" {
		request.Header.Set("Authorization", "Bearer "+h.config.BitbucketToken)
	} else if h.config.BitbucketUsername != "" && h.config.BitbucketPassword != "" {
		request.SetBasicAuth(h.config.BitbucketUsername, h.config.BitbucketPassword)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("create pull request call failed: %w", err)
	}
	defer response.Body.Close()

	var parsed struct {
		Id any `json:"id"`
	}
	if decodeErr := json.NewDecoder(response.Body).Decode(&parsed); decodeErr != nil {
		return "", fmt.Errorf("decode pull request response: %w", decodeErr)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("create pull request returned status %d", response.StatusCode)
	}

	if parsed.Id == nil {
		return "", fmt.Errorf("create pull request response missing id")
	}

	return fmt.Sprintf("%v", parsed.Id), nil
}

func (h *liveHarness) listCommitIDs(ctx context.Context, projectKey, repositorySlug string, limit int) ([]string, error) {
	var lastStatus int
	for attempt := 0; attempt < 8; attempt++ {
		limitValue := float32(limit)
		response, err := h.client.GetCommitsWithResponse(ctx, projectKey, repositorySlug, &openapigenerated.GetCommitsParams{Limit: &limitValue})
		if err != nil {
			return nil, fmt.Errorf("list commits call failed: %w", err)
		}

		lastStatus = response.StatusCode()
		if response.StatusCode() >= 200 && response.StatusCode() < 300 && response.ApplicationjsonCharsetUTF8200 != nil && response.ApplicationjsonCharsetUTF8200.Values != nil {
			ids := make([]string, 0, len(*response.ApplicationjsonCharsetUTF8200.Values))
			for _, value := range *response.ApplicationjsonCharsetUTF8200.Values {
				if value.Id != nil && *value.Id != "" {
					ids = append(ids, *value.Id)
				}
			}

			if len(ids) > 0 {
				return ids, nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("no commit ids found for %s/%s after retries (last status=%d)", projectKey, repositorySlug, lastStatus)
}

func (h *liveHarness) cleanupProject(ctx context.Context, seeded seededProject) error {
	for _, repo := range seeded.Repos {
		_, _ = h.client.DeleteRepositoryWithResponse(ctx, seeded.Key, repo.Slug)
	}

	_, _ = h.client.DeleteProjectWithResponse(ctx, seeded.Key)
	return nil
}

func repositoryPushURL(cfg config.AppConfig, projectKey, repositorySlug string) (string, error) {
	parsed, err := url.Parse(cfg.BitbucketURL)
	if err != nil {
		return "", fmt.Errorf("parse bitbucket url: %w", err)
	}
	parsed.User = url.UserPassword(cfg.BitbucketUsername, cfg.BitbucketPassword)
	parsed.Path = path.Join(parsed.Path, "scm", strings.ToUpper(projectKey), repositorySlug+".git")
	return parsed.String(), nil
}

func runGit(directory string, args ...string) error {
	command := exec.Command("git", args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
