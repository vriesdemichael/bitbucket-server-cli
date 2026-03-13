package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/git"
)

type cloneBackendStub struct {
	cloneCalls []cloneCall
	addCalls   []addRemoteCall
	cloneErr   error
	addErr     error
}

type cloneCall struct {
	repositoryURL string
	options       git.CloneOptions
}

type addRemoteCall struct {
	repositoryDirectory string
	remote              git.Remote
}

func (stub *cloneBackendStub) Version(context.Context) (string, error) {
	return "", nil
}

func (stub *cloneBackendStub) Clone(_ context.Context, repositoryURL string, options git.CloneOptions) error {
	stub.cloneCalls = append(stub.cloneCalls, cloneCall{repositoryURL: repositoryURL, options: options})
	if stub.cloneErr != nil {
		return stub.cloneErr
	}
	return nil
}

func (stub *cloneBackendStub) AddRemote(_ context.Context, repositoryDirectory string, remote git.Remote) error {
	stub.addCalls = append(stub.addCalls, addRemoteCall{repositoryDirectory: repositoryDirectory, remote: remote})
	if stub.addErr != nil {
		return stub.addErr
	}
	return nil
}

func (stub *cloneBackendStub) Fetch(context.Context, string, git.FetchOptions) error {
	return nil
}

func (stub *cloneBackendStub) Checkout(context.Context, string, git.CheckoutOptions) error {
	return nil
}

func (stub *cloneBackendStub) RepositoryRoot(context.Context, string) (string, error) {
	return "", nil
}

func (stub *cloneBackendStub) ListRemotes(context.Context, string) ([]git.Remote, error) {
	return nil, nil
}

func TestRepoCloneCommandClonesWithDefaults(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "https://bitbucket.example.com")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	output, err := executeTestCLI(t, "repo", "clone", "PRJ/demo")
	if err != nil {
		t.Fatalf("repo clone failed: %v", err)
	}

	if len(stub.cloneCalls) != 1 {
		t.Fatalf("expected one clone call, got %d", len(stub.cloneCalls))
	}

	call := stub.cloneCalls[0]
	if call.repositoryURL != "https://bitbucket.example.com/scm/PRJ/demo.git" {
		t.Fatalf("unexpected clone url: %s", call.repositoryURL)
	}
	if call.options.Directory != "demo" {
		t.Fatalf("unexpected clone directory: %s", call.options.Directory)
	}
	if len(call.options.ExtraArgs) != 0 {
		t.Fatalf("expected no extra git args, got: %#v", call.options.ExtraArgs)
	}

	if !strings.Contains(output, "Cloned PRJ/demo into demo") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestRepoCloneCommandHonorsDirectoryAndGitFlags(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "https://bitbucket.example.com/bitbucket")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	output, err := executeTestCLI(t, "--json", "repo", "clone", "PRJ/demo", "target-dir", "--", "--depth=7", "--branch=main")
	if err != nil {
		t.Fatalf("repo clone json failed: %v", err)
	}

	if len(stub.cloneCalls) != 1 {
		t.Fatalf("expected one clone call, got %d", len(stub.cloneCalls))
	}

	call := stub.cloneCalls[0]
	if call.repositoryURL != "https://bitbucket.example.com/bitbucket/scm/PRJ/demo.git" {
		t.Fatalf("unexpected clone url: %s", call.repositoryURL)
	}
	if call.options.Directory != "target-dir" {
		t.Fatalf("unexpected clone directory: %s", call.options.Directory)
	}
	if len(call.options.ExtraArgs) != 2 {
		t.Fatalf("unexpected extra args: %#v", call.options.ExtraArgs)
	}
	if call.options.ExtraArgs[0] != "--depth=7" || call.options.ExtraArgs[1] != "--branch=main" {
		t.Fatalf("unexpected extra args: %#v", call.options.ExtraArgs)
	}

	if !strings.Contains(output, `"clone_url": "https://bitbucket.example.com/bitbucket/scm/PRJ/demo.git"`) {
		t.Fatalf("unexpected json output: %s", output)
	}
}

func TestRepoCloneCommandValidationAndBackendFailure(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{cloneErr: errors.New("clone failed")}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "https://bitbucket.example.com")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	_, err := executeTestCLI(t, "repo", "clone", "badformat")
	if err == nil {
		t.Fatal("expected invalid selector error")
	}

	_, err = executeTestCLI(t, "repo", "clone", "PRJ/demo", "")
	if err == nil {
		t.Fatal("expected empty directory validation error")
	}

	_, err = executeTestCLI(t, "repo", "clone", "PRJ/demo", "target-dir", "--", "depth=1")
	if err == nil {
		t.Fatal("expected invalid extra git args error")
	}

	_, err = executeTestCLI(t, "repo", "clone", "PRJ/demo")
	if err == nil {
		t.Fatal("expected backend clone failure")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Fatalf("expected backend error in result, got: %v", err)
	}
}

func TestRepoCloneCommandSupportsURLSelectors(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "https://bitbucket.example.com")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	_, err := executeTestCLI(t, "repo", "clone", "https://bitbucket.other.example/scm/OPS/tooling.git")
	if err != nil {
		t.Fatalf("repo clone with URL selector failed: %v", err)
	}

	if len(stub.cloneCalls) != 1 {
		t.Fatalf("expected one clone call, got %d", len(stub.cloneCalls))
	}

	call := stub.cloneCalls[0]
	if call.repositoryURL != "https://bitbucket.other.example/scm/OPS/tooling.git" {
		t.Fatalf("unexpected clone URL: %s", call.repositoryURL)
	}
	if call.options.Directory != "tooling" {
		t.Fatalf("unexpected clone directory: %s", call.options.Directory)
	}
}

func TestRepoCloneCommandNoUpstreamAndAddRemoteFailure(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{addErr: errors.New("add remote failed")}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/rest/api/1.0/projects/PRJ/repos/demo" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"origin":{"project":{"key":"UP"},"slug":"upstream-demo"}}`))
			return
		}
		http.NotFound(writer, request)
	}))
	t.Cleanup(server.Close)

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	_, err := executeTestCLI(t, "repo", "clone", "--no-upstream", "PRJ/demo")
	if err != nil {
		t.Fatalf("repo clone with --no-upstream failed: %v", err)
	}
	if len(stub.addCalls) != 0 {
		t.Fatalf("expected no add-remote calls with --no-upstream, got: %#v", stub.addCalls)
	}

	_, err = executeTestCLI(t, "repo", "clone", "--upstream-remote-name", "@owner", "PRJ/demo")
	if err == nil {
		t.Fatal("expected add-remote failure")
	}
	if !strings.Contains(err.Error(), "add remote failed") {
		t.Fatalf("unexpected add-remote error: %v", err)
	}
	if len(stub.addCalls) == 0 {
		t.Fatal("expected add-remote to be attempted")
	}
	if stub.addCalls[len(stub.addCalls)-1].remote.Name != "up" {
		t.Fatalf("expected @owner to resolve to project key remote name, got: %s", stub.addCalls[len(stub.addCalls)-1].remote.Name)
	}
}

func TestBuildBitbucketCloneURL(t *testing.T) {
	cloneURL, err := buildBitbucketCloneURL("https://bitbucket.example.com/context", "PRJ", "demo")
	if err != nil {
		t.Fatalf("expected valid clone url, got: %v", err)
	}
	if cloneURL != "https://bitbucket.example.com/context/scm/PRJ/demo.git" {
		t.Fatalf("unexpected clone url: %s", cloneURL)
	}

	_, err = buildBitbucketCloneURL("bad-url", "PRJ", "demo")
	if err == nil {
		t.Fatal("expected invalid base URL error")
	}
}

func TestResolveRepositoryCloneInputParsesSelectors(t *testing.T) {
	cfg := config.AppConfig{BitbucketURL: "https://bitbucket.example.com", ProjectKey: "PRJ"}

	repo, host, usedURL, err := resolveRepositoryCloneInput("PRJ/repo", cfg)
	if err != nil {
		t.Fatalf("parse PROJECT/slug failed: %v", err)
	}
	if repo.ProjectKey != "PRJ" || repo.Slug != "repo" || host != "https://bitbucket.example.com" || usedURL {
		t.Fatalf("unexpected PROJECT/slug parse result: %+v host=%s usedURL=%v", repo, host, usedURL)
	}

	repo, host, usedURL, err = resolveRepositoryCloneInput("bb.company.local/OPS/tooling", cfg)
	if err != nil {
		t.Fatalf("parse host-qualified selector failed: %v", err)
	}
	if repo.ProjectKey != "OPS" || repo.Slug != "tooling" || host != "https://bb.company.local" || !usedURL {
		t.Fatalf("unexpected host-qualified parse result: %+v host=%s usedURL=%v", repo, host, usedURL)
	}

	repo, host, usedURL, err = resolveRepositoryCloneInput("https://bb.company.local/scm/OPS/tooling.git", cfg)
	if err != nil {
		t.Fatalf("parse URL selector failed: %v", err)
	}
	if repo.ProjectKey != "OPS" || repo.Slug != "tooling" || host != "https://bb.company.local" || !usedURL {
		t.Fatalf("unexpected URL parse result: %+v host=%s usedURL=%v", repo, host, usedURL)
	}
}

func TestCloneHelpersAndValidation(t *testing.T) {
	dir, extra := splitCloneDirectoryAndExtraArgs("repo", []string{"target", "--", "--depth=1"})
	if dir != "target" || len(extra) != 2 {
		t.Fatalf("unexpected split result: dir=%q extra=%#v", dir, extra)
	}

	dir, extra = splitCloneDirectoryAndExtraArgs("repo", []string{"--", "--depth=1"})
	if dir != "repo" || len(extra) != 2 {
		t.Fatalf("unexpected split without explicit directory: dir=%q extra=%#v", dir, extra)
	}

	args, err := normalizeCloneExtraArgs([]string{"--", "--depth=1", "--branch=main"})
	if err != nil {
		t.Fatalf("normalize clone extra args failed: %v", err)
	}
	if len(args) != 2 {
		t.Fatalf("unexpected normalized args: %#v", args)
	}

	_, err = normalizeCloneExtraArgs([]string{"depth=1"})
	if err == nil {
		t.Fatal("expected validation error when extra arg is not a flag")
	}

	name, err := normalizeUpstreamRemoteName("@owner", "OPS")
	if err != nil || name != "ops" {
		t.Fatalf("unexpected @owner mapping: name=%q err=%v", name, err)
	}

	_, err = normalizeUpstreamRemoteName("bad name", "OPS")
	if err == nil {
		t.Fatal("expected validation error for invalid remote name")
	}

	name, err = normalizeUpstreamRemoteName("@owner", "")
	if err != nil || name != "owner" {
		t.Fatalf("unexpected @owner fallback mapping: name=%q err=%v", name, err)
	}
}

func TestLookupParentCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/rest/api/1.0/projects/PRJ/repos/demo":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"origin":{"project":{"key":"UP"},"slug":"upstream-demo"}}`))
		case "/rest/api/1.0/projects/PRJ/repos/no-origin":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"slug":"no-origin"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)

	cfg := config.AppConfig{BitbucketURL: server.URL}

	owner, parentURL, err := lookupParentCloneURL(context.Background(), cfg, server.URL, repositorySelector{ProjectKey: "PRJ", Slug: "demo"})
	if err != nil {
		t.Fatalf("lookup parent clone URL failed: %v", err)
	}
	if owner != "UP" {
		t.Fatalf("unexpected owner: %s", owner)
	}
	if !strings.Contains(parentURL, "/scm/UP/upstream-demo.git") {
		t.Fatalf("unexpected parent clone URL: %s", parentURL)
	}

	owner, parentURL, err = lookupParentCloneURL(context.Background(), cfg, server.URL, repositorySelector{ProjectKey: "PRJ", Slug: "no-origin"})
	if err != nil {
		t.Fatalf("lookup parent clone URL without origin failed: %v", err)
	}
	if owner != "" || parentURL != "" {
		t.Fatalf("expected no parent URL when origin missing, got owner=%q url=%q", owner, parentURL)
	}

	owner, parentURL, err = lookupParentCloneURL(context.Background(), cfg, server.URL, repositorySelector{ProjectKey: "PRJ", Slug: "does-not-exist"})
	if err != nil {
		t.Fatalf("expected graceful lookup when repo not found, got: %v", err)
	}
	if owner != "" || parentURL != "" {
		t.Fatalf("expected empty parent on lookup failure, got owner=%q url=%q", owner, parentURL)
	}
}

func TestRepoCloneCommandJSONAndUpstreamSuccess(t *testing.T) {
	originalFactory := gitBackendFactory
	stub := &cloneBackendStub{}
	gitBackendFactory = func() git.Backend { return stub }
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/rest/api/1.0/projects/PRJ/repos/demo" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"origin":{"project":{"key":"UP"},"slug":"upstream-demo"}}`))
			return
		}
		http.NotFound(writer, request)
	}))
	t.Cleanup(server.Close)

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", server.URL)
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "demo")

	output, err := executeTestCLI(t, "--json", "repo", "clone", "PRJ/demo")
	if err != nil {
		t.Fatalf("repo clone json failed: %v", err)
	}

	if len(stub.addCalls) != 1 {
		t.Fatalf("expected add remote call, got %d", len(stub.addCalls))
	}
	if stub.addCalls[0].remote.Name != "upstream" {
		t.Fatalf("unexpected upstream remote name: %s", stub.addCalls[0].remote.Name)
	}
	if !strings.Contains(stub.addCalls[0].remote.URL, "/scm/UP/upstream-demo.git") {
		t.Fatalf("unexpected upstream remote URL: %s", stub.addCalls[0].remote.URL)
	}
	if !strings.Contains(output, `"configured": true`) {
		t.Fatalf("expected upstream configured in json output, got: %s", output)
	}
}

func TestRepoCloneCommandConfigAndFactoryValidation(t *testing.T) {
	originalFactory := gitBackendFactory
	t.Cleanup(func() { gitBackendFactory = originalFactory })

	_, _, _, err := resolveRepositoryCloneInput("demo", config.AppConfig{})
	if err == nil {
		t.Fatal("expected config validation error for slug-only clone without project")
	}

	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "https://bitbucket.example.com")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")

	gitBackendFactory = func() git.Backend { return nil }
	_, err = executeTestCLI(t, "repo", "clone", "PRJ/demo")
	if err == nil {
		t.Fatal("expected git backend factory validation error")
	}
	if !strings.Contains(err.Error(), "git backend is not configured") {
		t.Fatalf("unexpected backend validation error: %v", err)
	}

	t.Setenv("BITBUCKET_URL", "://bad-url")
	gitBackendFactory = func() git.Backend { return &cloneBackendStub{} }
	_, err = executeTestCLI(t, "repo", "clone", "PRJ/demo")
	if err == nil {
		t.Fatal("expected clone URL validation error")
	}
}

func TestResolveRepositoryCloneInputValidationBranches(t *testing.T) {
	_, _, _, err := resolveRepositoryCloneInput("", config.AppConfig{BitbucketURL: "https://bitbucket.example.com", ProjectKey: "PRJ"})
	if err == nil {
		t.Fatal("expected empty repository validation error")
	}

	_, _, _, err = resolveRepositoryCloneInput("PRJ/repo", config.AppConfig{})
	if err == nil {
		t.Fatal("expected missing host validation for PROJECT/slug")
	}

	_, _, _, err = resolveRepositoryCloneInput("bad/", config.AppConfig{BitbucketURL: "https://bitbucket.example.com", ProjectKey: "PRJ"})
	if err == nil {
		t.Fatal("expected invalid selector error with slash")
	}
}

func TestNormalizeCloneHostFallback(t *testing.T) {
	host := normalizeCloneHost("git@bb.example.local:OPS/tooling.git", "bb.example.local")
	if host != "https://bb.example.local" {
		t.Fatalf("unexpected normalized host: %s", host)
	}

	host = normalizeCloneHost("invalid", "")
	if host != "" {
		t.Fatalf("expected empty host for empty parsed host, got: %s", host)
	}
}

func TestBuildBitbucketCloneURLEmptySelectorValidation(t *testing.T) {
	_, err := buildBitbucketCloneURL("https://bitbucket.example.com", "", "demo")
	if err == nil {
		t.Fatal("expected empty project validation error")
	}

	_, err = buildBitbucketCloneURL("https://bitbucket.example.com", "PRJ", "")
	if err == nil {
		t.Fatal("expected empty slug validation error")
	}
}
