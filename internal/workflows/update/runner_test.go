package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	githubrelease "github.com/vriesdemichael/bitbucket-server-cli/internal/transport/githubrelease"
	updatesigstore "github.com/vriesdemichael/bitbucket-server-cli/internal/transport/sigstore"
)

type stubReleaseClient struct {
	release       githubrelease.Release
	latestErr     error
	downloads     map[string][]byte
	downloadErrs  map[string]error
	latestCalls   int
	downloadCalls []string
}

func (stub *stubReleaseClient) Latest(context.Context, string, string) (githubrelease.Release, error) {
	stub.latestCalls++
	return stub.release, stub.latestErr
}

func (stub *stubReleaseClient) Download(_ context.Context, assetURL string) ([]byte, error) {
	stub.downloadCalls = append(stub.downloadCalls, assetURL)
	if err := stub.downloadErrs[assetURL]; err != nil {
		return nil, err
	}
	return stub.downloads[assetURL], nil
}

type stubSignatureVerifier struct {
	verification updatesigstore.Verification
	err          error
	calls        int
}

func (stub *stubSignatureVerifier) VerifyBlob(context.Context, []byte, []byte) (updatesigstore.Verification, error) {
	stub.calls++
	if stub.err != nil {
		return updatesigstore.Verification{}, stub.err
	}
	if stub.verification.CertificateIdentity == "" {
		stub.verification = updatesigstore.Verification{
			CertificateIdentity:            "https://github.com/vriesdemichael/bitbucket-server-cli/.github/workflows/release.yml@refs/heads/main",
			CertificateOIDCIssuer:          updatesigstore.GitHubActionsIssuer,
			TransparencyLogEntriesVerified: 1,
			VerifiedTimestampCount:         1,
		}
	}
	return stub.verification, nil
}

func newTestRunner(deps Dependencies) *Runner {
	if deps.Verifier == nil {
		deps.Verifier = &stubSignatureVerifier{}
	}
	return NewRunner(deps)
}

func releaseWithSignatureBundle(release githubrelease.Release) githubrelease.Release {
	for _, asset := range release.Assets {
		if asset.Name == "sha256sums.txt" {
			release.Assets = append(release.Assets, githubrelease.Asset{Name: "sha256sums.txt.sigstore.json", BrowserDownloadURL: "bundle"})
			break
		}
	}
	return release
}

func downloadsWithSignatureBundle(downloads map[string][]byte) map[string][]byte {
	if downloads == nil {
		downloads = map[string][]byte{}
	}
	if _, ok := downloads["bundle"]; !ok {
		downloads["bundle"] = []byte("signed-bundle")
	}
	return downloads
}

func TestRunnerDryRunPlansUpdateWithoutWritingBinary(t *testing.T) {
	archive := buildTarGzArchive(t, "bb", []byte("new-binary"))
	checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_linux_amd64.tar.gz")

	client := &stubReleaseClient{
		release: releaseWithSignatureBundle(githubrelease.Release{
			TagName: "v1.2.0",
			HTMLURL: "https://example.test/releases/v1.2.0",
			Assets: []githubrelease.Asset{
				{Name: "bb_1.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.test/bb_1.2.0_linux_amd64.tar.gz"},
				{Name: "sha256sums.txt", BrowserDownloadURL: "https://example.test/sha256sums.txt"},
			},
		}),
		downloads: downloadsWithSignatureBundle(map[string][]byte{
			"https://example.test/sha256sums.txt": []byte(checksum),
		}),
	}

	written := false
	runner := newTestRunner(Dependencies{
		Releases:        client,
		RepositoryOwner: "vriesdemichael",
		RepositoryName:  "bitbucket-server-cli",
		CurrentVersion:  func() string { return "v1.1.0" },
		ExecutablePath:  func() (string, error) { return "/tmp/bb", nil },
		Platform:        func() (string, string) { return "linux", "amd64" },
		WriteBinary: func(string, []byte, fs.FileMode) error {
			written = true
			return nil
		},
	})

	result, err := runner.Run(context.Background(), Options{DryRun: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if written {
		t.Fatal("expected dry-run not to write binary")
	}
	if !result.UpdateAvailable || result.Applied || !result.DryRun {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.ChecksumAvailable || result.ChecksumVerified {
		t.Fatalf("expected checksum to be available but not verified, got %+v", result)
	}
	if len(client.downloadCalls) != 2 || client.downloadCalls[0] != "https://example.test/sha256sums.txt" || client.downloadCalls[1] != "bundle" {
		t.Fatalf("unexpected dry-run downloads: %+v", client.downloadCalls)
	}
}

func TestRunnerAppliesReleaseUpdate(t *testing.T) {
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "bb")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write initial target: %v", err)
	}

	archive := buildTarGzArchive(t, "bb", []byte("new-binary"))
	checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_linux_amd64.tar.gz")

	client := &stubReleaseClient{
		release: releaseWithSignatureBundle(githubrelease.Release{
			TagName: "v1.2.0",
			HTMLURL: "https://example.test/releases/v1.2.0",
			Assets: []githubrelease.Asset{
				{Name: "bb_1.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.test/bb_1.2.0_linux_amd64.tar.gz"},
				{Name: "sha256sums.txt", BrowserDownloadURL: "https://example.test/sha256sums.txt"},
			},
		}),
		downloads: downloadsWithSignatureBundle(map[string][]byte{
			"https://example.test/sha256sums.txt":              []byte(checksum),
			"https://example.test/bb_1.2.0_linux_amd64.tar.gz": archive,
		}),
	}

	runner := newTestRunner(Dependencies{
		Releases:        client,
		RepositoryOwner: "vriesdemichael",
		RepositoryName:  "bitbucket-server-cli",
		CurrentVersion:  func() string { return "v1.1.0" },
		ExecutablePath:  func() (string, error) { return targetPath, nil },
		Platform:        func() (string, string) { return "linux", "amd64" },
	})

	result, err := runner.Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Applied || !result.ChecksumVerified {
		t.Fatalf("expected applied verified result, got %+v", result)
	}
	updated, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read updated target: %v", err)
	}
	if string(updated) != "new-binary" {
		t.Fatalf("expected updated binary contents, got %q", string(updated))
	}
	if len(client.downloadCalls) != 3 {
		t.Fatalf("expected two downloads, got %+v", client.downloadCalls)
	}
}

func TestRunnerReturnsUpToDateWithoutDownloads(t *testing.T) {
	client := &stubReleaseClient{
		release:   githubrelease.Release{TagName: "v1.2.0", HTMLURL: "https://example.test/releases/v1.2.0"},
		downloads: map[string][]byte{},
	}

	runner := newTestRunner(Dependencies{
		Releases:        client,
		RepositoryOwner: "vriesdemichael",
		RepositoryName:  "bitbucket-server-cli",
		CurrentVersion:  func() string { return "v1.2.0" },
		ExecutablePath:  func() (string, error) { return "/tmp/bb", nil },
		Platform:        func() (string, string) { return "linux", "amd64" },
	})

	result, err := runner.Run(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.UpToDate || result.UpdateAvailable {
		t.Fatalf("expected up-to-date result, got %+v", result)
	}
	if len(client.downloadCalls) != 0 {
		t.Fatalf("expected no downloads when already current, got %+v", client.downloadCalls)
	}
}

func buildTarGzArchive(t *testing.T, fileName string, contents []byte) []byte {
	t.Helper()
	return buildTarGzArchiveWithMode(t, fileName, contents, 0o755)
}

func buildTarGzArchiveWithMode(t *testing.T, fileName string, contents []byte, mode int64) []byte {
	t.Helper()

	buffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{Name: fileName, Mode: mode, Size: int64(len(contents))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	return buffer.Bytes()
}

func buildZipArchive(t *testing.T, fileName string, contents []byte) []byte {
	t.Helper()
	return buildZipArchiveWithMode(t, fileName, contents, 0)
}

func buildZipArchiveWithMode(t *testing.T, fileName string, contents []byte, mode fs.FileMode) []byte {
	t.Helper()

	buffer := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buffer)
	header := &zip.FileHeader{Name: fileName}
	header.SetMode(mode)
	fileWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := fileWriter.Write(contents); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	return buffer.Bytes()
}

func TestRunnerValidationAndErrorPaths(t *testing.T) {
	t.Run("runner not configured", func(t *testing.T) {
		var runner *Runner
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("repository not configured", func(t *testing.T) {
		runner := newTestRunner(Dependencies{Releases: &stubReleaseClient{}})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("latest release failure", func(t *testing.T) {
		runner := newTestRunner(Dependencies{
			Releases:        &stubReleaseClient{latestErr: apperrors.New(apperrors.KindTransient, "boom", nil)},
			RepositoryOwner: "vriesdemichael",
			RepositoryName:  "bitbucket-server-cli",
		})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindTransient) {
			t.Fatalf("expected transient error, got %v", err)
		}
	})

	t.Run("missing latest tag", func(t *testing.T) {
		runner := newTestRunner(Dependencies{
			Releases:        &stubReleaseClient{release: githubrelease.Release{}},
			RepositoryOwner: "vriesdemichael",
			RepositoryName:  "bitbucket-server-cli",
		})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("invalid latest semver", func(t *testing.T) {
		runner := newTestRunner(Dependencies{
			Releases:        &stubReleaseClient{release: githubrelease.Release{TagName: "latest"}},
			RepositoryOwner: "vriesdemichael",
			RepositoryName:  "bitbucket-server-cli",
		})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("missing executable path", func(t *testing.T) {
		runner := newTestRunner(Dependencies{
			Releases:        &stubReleaseClient{release: githubrelease.Release{TagName: "v1.2.0"}},
			RepositoryOwner: "vriesdemichael",
			RepositoryName:  "bitbucket-server-cli",
			ExecutablePath: func() (string, error) {
				return "", os.ErrNotExist
			},
		})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})
}

func TestRunnerUpdateErrorCases(t *testing.T) {
	baseRelease := releaseWithSignatureBundle(githubrelease.Release{
		TagName: "v1.2.0",
		Assets: []githubrelease.Asset{{Name: "bb_1.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "archive"}, {Name: "sha256sums.txt", BrowserDownloadURL: "checksums"}},
	})

	t.Run("missing archive asset", func(t *testing.T) {
		client := &stubReleaseClient{release: releaseWithSignatureBundle(githubrelease.Release{TagName: "v1.2.0", Assets: []githubrelease.Asset{{Name: "sha256sums.txt", BrowserDownloadURL: "checksums"}}})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindNotFound) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("missing checksum asset", func(t *testing.T) {
		client := &stubReleaseClient{release: githubrelease.Release{TagName: "v1.2.0", Assets: []githubrelease.Asset{{Name: "bb_1.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "archive"}}}}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindNotFound) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("missing signature bundle asset", func(t *testing.T) {
		client := &stubReleaseClient{release: baseRelease}
		client.release.Assets = client.release.Assets[:2]
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindNotFound) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("missing checksum entry", func(t *testing.T) {
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte("deadbeef  other.tar.gz\n")})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("checksum download failure", func(t *testing.T) {
		client := &stubReleaseClient{release: baseRelease, downloadErrs: map[string]error{"checksums": apperrors.New(apperrors.KindTransient, "download failed", nil)}}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindTransient) {
			t.Fatalf("expected transient error, got %v", err)
		}
	})

	t.Run("signature bundle download failure", func(t *testing.T) {
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(nil), downloadErrs: map[string]error{"bundle": apperrors.New(apperrors.KindTransient, "download failed", nil)}}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindTransient) {
			t.Fatalf("expected transient error, got %v", err)
		}
	})

	t.Run("signature verification failure", func(t *testing.T) {
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte("deadbeef  bb_1.2.0_linux_amd64.tar.gz\n")})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }, Verifier: &stubSignatureVerifier{err: apperrors.New(apperrors.KindPermanent, "bad signature", nil)}})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		archive := buildTarGzArchive(t, "bb", []byte("new-binary"))
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte("deadbeef  bb_1.2.0_linux_amd64.tar.gz\n"), "archive": archive})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("archive download failure", func(t *testing.T) {
		archive := buildTarGzArchive(t, "bb", []byte("new-binary"))
		checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_linux_amd64.tar.gz")
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte(checksum)}), downloadErrs: map[string]error{"archive": apperrors.New(apperrors.KindTransient, "download failed", nil)}}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindTransient) {
			t.Fatalf("expected transient error, got %v", err)
		}
	})

	t.Run("archive extraction failure", func(t *testing.T) {
		archive := []byte("not-an-archive")
		checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_linux_amd64.tar.gz")
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte(checksum), "archive": archive})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got %v", err)
		}
	})

	t.Run("write binary error", func(t *testing.T) {
		archive := buildTarGzArchive(t, "bb", []byte("new-binary"))
		checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_linux_amd64.tar.gz")
		client := &stubReleaseClient{release: baseRelease, downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte(checksum), "archive": archive})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }, WriteBinary: func(string, []byte, fs.FileMode) error { return apperrors.New(apperrors.KindInternal, "write failed", nil) }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})
}

func TestRunnerWindowsAndVersionComparisonPaths(t *testing.T) {
	t.Run("windows zip update", func(t *testing.T) {
		archive := buildZipArchive(t, "bb.exe", []byte("windows-binary"))
		checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_windows_amd64.zip")
		client := &stubReleaseClient{release: releaseWithSignatureBundle(githubrelease.Release{TagName: "v1.2.0", Assets: []githubrelease.Asset{{Name: "bb_1.2.0_windows_amd64.zip", BrowserDownloadURL: "archive"}, {Name: "sha256sums.txt", BrowserDownloadURL: "checksums"}}}), downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte(checksum), "archive": archive})}
		targetPath := filepath.Join(t.TempDir(), "bb.exe")
		if err := os.WriteFile(targetPath, []byte("old"), 0o755); err != nil {
			t.Fatalf("seed target: %v", err)
		}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return targetPath, nil }, Platform: func() (string, string) { return "windows", "amd64" }})
		result, err := runner.Run(context.Background(), Options{DryRun: true})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if !result.UpdateAvailable || result.PlannedAction != "download_and_replace_after_exit" {
			t.Fatalf("expected dry-run windows plan, got %+v", result)
		}
	})

	t.Run("windows apply returns manual replacement error", func(t *testing.T) {
		archive := buildZipArchive(t, "bb.exe", []byte("windows-binary"))
		checksum := fmt.Sprintf("%s  %s\n", sha256Hex(archive), "bb_1.2.0_windows_amd64.zip")
		client := &stubReleaseClient{release: releaseWithSignatureBundle(githubrelease.Release{TagName: "v1.2.0", Assets: []githubrelease.Asset{{Name: "bb_1.2.0_windows_amd64.zip", BrowserDownloadURL: "archive"}, {Name: "sha256sums.txt", BrowserDownloadURL: "checksums"}}}), downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte(checksum), "archive": archive})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.1.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb.exe", nil }, Platform: func() (string, string) { return "windows", "amd64" }})
		_, err := runner.Run(context.Background(), Options{})
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent windows replacement error, got %v", err)
		}
	})

	t.Run("current newer", func(t *testing.T) {
		client := &stubReleaseClient{release: githubrelease.Release{TagName: "v1.2.0"}}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "v1.3.0" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }})
		result, err := runner.Run(context.Background(), Options{})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if result.UpdateAvailable || result.Comparison != "current_newer" {
			t.Fatalf("expected current_newer result, got %+v", result)
		}
	})

	t.Run("unknown current version", func(t *testing.T) {
		client := &stubReleaseClient{release: releaseWithSignatureBundle(githubrelease.Release{TagName: "v1.2.0", Assets: []githubrelease.Asset{{Name: "bb_1.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "archive"}, {Name: "sha256sums.txt", BrowserDownloadURL: "checksums"}}}), downloads: downloadsWithSignatureBundle(map[string][]byte{"checksums": []byte("deadbeef  bb_1.2.0_linux_amd64.tar.gz\n")})}
		runner := newTestRunner(Dependencies{Releases: client, RepositoryOwner: "vriesdemichael", RepositoryName: "bitbucket-server-cli", CurrentVersion: func() string { return "dev" }, ExecutablePath: func() (string, error) { return "/tmp/bb", nil }, Platform: func() (string, string) { return "linux", "amd64" }})
		result, err := runner.Run(context.Background(), Options{DryRun: true})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if !result.UpdateAvailable || result.Comparison != "unknown_current" {
			t.Fatalf("expected unknown_current result, got %+v", result)
		}
	})
}

func TestUpdateHelpers(t *testing.T) {
	if got := archiveName("v1.2.3", "linux", "amd64"); got != "bb_1.2.3_linux_amd64.tar.gz" {
		t.Fatalf("unexpected archive name: %s", got)
	}
	if got := archiveName("v1.2.3", "windows", "arm64"); got != "bb_1.2.3_windows_arm64.zip" {
		t.Fatalf("unexpected archive name: %s", got)
	}
	if binaryFileName("windows") != "bb.exe" || binaryFileName("linux") != "bb" {
		t.Fatal("unexpected binary file names")
	}
	if _, ok := findAsset([]githubrelease.Asset{{Name: "a"}}, "a"); !ok {
		t.Fatal("expected asset to be found")
	}
	if _, ok := findAsset([]githubrelease.Asset{{Name: "a"}}, "b"); ok {
		t.Fatal("expected asset miss")
	}

	checksums, err := parseChecksums([]byte("deadbeef  file.tar.gz\n"))
	if err != nil || checksums["file.tar.gz"] != "deadbeef" {
		t.Fatalf("unexpected checksums parse result: %+v %v", checksums, err)
	}
	checksums, err = parseChecksums([]byte("deadbeef  ./bb_1.2.0_linux_amd64.tar.gz\n"))
	if err != nil || checksums["bb_1.2.0_linux_amd64.tar.gz"] != "deadbeef" {
		t.Fatalf("expected normalized checksum file name, got %+v %v", checksums, err)
	}
	if _, err := parseChecksums([]byte("broken-line")); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected malformed checksum error, got %v", err)
	}
	if _, err := parseChecksums([]byte("\n")); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected empty checksum error, got %v", err)
	}

	tarArchive := buildTarGzArchive(t, "bb", []byte("payload"))
	if extracted, _, err := extractBinary("archive.tar.gz", "bb", tarArchive); err != nil || string(extracted) != "payload" {
		t.Fatalf("unexpected tar extract result: %q %v", string(extracted), err)
	}
	tarDefaultMode := buildTarGzArchive(t, "bb", []byte("payload"))
	if _, mode, err := extractBinaryFromTarGz("bb", tarDefaultMode); err != nil || mode == 0 {
		t.Fatalf("expected tar mode, got %o %v", mode, err)
	}
	if _, mode, err := extractBinaryFromZip("bb.exe", buildZipArchive(t, "bb.exe", []byte("payload"))); err != nil || mode != 0o755 {
		t.Fatalf("expected default zip mode, got %o %v", mode, err)
	}
	zipArchive := buildZipArchive(t, "bb.exe", []byte("payload"))
	if extracted, _, err := extractBinary("archive.zip", "bb.exe", zipArchive); err != nil || string(extracted) != "payload" {
		t.Fatalf("unexpected zip extract result: %q %v", string(extracted), err)
	}
	if _, _, err := extractBinaryFromZip("bb.exe", buildZipArchive(t, "other.exe", []byte("payload"))); !apperrors.IsKind(err, apperrors.KindNotFound) {
		t.Fatalf("expected missing zip binary error, got %v", err)
	}
	invalidTarBuffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(invalidTarBuffer)
	if _, err := gzipWriter.Write([]byte("not-a-tar-stream")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if _, _, err := extractBinaryFromTarGz("bb", invalidTarBuffer.Bytes()); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected tar read error, got %v", err)
	}
	if _, _, err := extractBinary("archive.tar.gz", "bb", []byte("not-a-tar")); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected bad tar error, got %v", err)
	}
	if _, _, err := extractBinary("archive.zip", "bb.exe", []byte("not-a-zip")); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected bad zip error, got %v", err)
	}
	if _, _, err := extractBinary("archive.bin", "bb", []byte("x")); !apperrors.IsKind(err, apperrors.KindPermanent) {
		t.Fatalf("expected unsupported archive error, got %v", err)
	}

	missingArchive := buildTarGzArchive(t, "other", []byte("payload"))
	if _, _, err := extractBinary("archive.tar.gz", "bb", missingArchive); !apperrors.IsKind(err, apperrors.KindNotFound) {
		t.Fatalf("expected missing binary error, got %v", err)
	}

	if normalizeSemver("1.2.3") != "v1.2.3" || normalizeSemver("v1.2.3-beta.1") != "v1.2.3-beta.1" || normalizeSemver("bad") != "" {
		t.Fatal("unexpected normalizeSemver results")
	}
	if normalizeSemver("  ") != "" {
		t.Fatal("expected empty normalized version")
	}
	if compareSemver("v1.2.3", "v1.2.4") >= 0 || compareSemver("v1.2.4", "v1.2.3") <= 0 || compareSemver("v1.2.3", "v1.2.3") != 0 {
		t.Fatal("unexpected compareSemver results")
	}
	if compareSemver("alpha", "beta") >= 0 {
		t.Fatal("expected fallback lexical comparison for invalid semver")
	}
	if compareSemver("v1.2.3-beta.1", "v1.2.3") >= 0 {
		t.Fatal("expected prerelease to sort before stable")
	}
	if comparePrerelease("alpha.1", "alpha.2") >= 0 || comparePrerelease("", "alpha") <= 0 || comparePrerelease("alpha", "") >= 0 || comparePrerelease("alpha.1", "alpha") <= 0 || comparePrerelease("alpha", "alpha") != 0 || compareIdentifier("1", "alpha") >= 0 || compareIdentifier("alpha", "1") <= 0 || compareIdentifier("alpha", "beta") >= 0 || compareIdentifier("2", "2") != 0 {
		t.Fatal("unexpected prerelease comparison results")
	}
	if compareInt(1, 2) >= 0 || compareInt(2, 1) <= 0 || compareInt(2, 2) != 0 {
		t.Fatal("unexpected compareInt results")
	}
	if update, comparison := isUpdateAvailable("v1.0.1", "v1.0.1", "v1.0.1", "v1.0.1"); update || comparison != "equal" {
		t.Fatalf("unexpected equal result: %v %s", update, comparison)
	}
	if update, comparison := isUpdateAvailable("dev", "", "dev", "v1.0.1"); update || comparison != "equal" {
		t.Fatalf("unexpected unknown-current equal result: %v %s", update, comparison)
	}
	if value, ok := parseSemver("v1.2.3+build.5"); !ok || value.original != "v1.2.3" {
		t.Fatalf("unexpected build metadata parse result: %+v %v", value, ok)
	}
	if _, ok := parseSemver("1.2.3"); ok {
		t.Fatal("expected missing-v semver parse failure")
	}
	if _, ok := parseSemver("v1.2"); ok {
		t.Fatal("expected short semver parse failure")
	}
	if _, ok := parseSemver("v1.x.3"); ok {
		t.Fatal("expected invalid major/minor semver parse failure")
	}
	if update, comparison := isUpdateAvailable("v1.0.0", "v1.0.0", "v1.0.1", "v1.0.1"); !update || comparison != "upgrade_available" {
		t.Fatalf("unexpected isUpdateAvailable result: %v %s", update, comparison)
	}
	if plannedAction("windows") != "download_and_replace_after_exit" || plannedAction("linux") != "replace" {
		t.Fatal("unexpected planned action values")
	}
	if files := SortedChecksumFiles(map[string]string{"b": "2", "a": "1"}); len(files) != 2 || files[0] != "a" || files[1] != "b" {
		t.Fatalf("unexpected sorted files: %+v", files)
	}
}

func TestReplaceBinary(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		if err := replaceBinary("", []byte("payload"), 0o755); !apperrors.IsKind(err, apperrors.KindValidation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("create temp failure", func(t *testing.T) {
		targetPath := filepath.Join(t.TempDir(), "missing", "bb")
		if err := replaceBinary(targetPath, []byte("payload"), 0o755); !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got %v", err)
		}
	})

	t.Run("successful replacement", func(t *testing.T) {
		targetDir := t.TempDir()
		targetPath := filepath.Join(targetDir, "bb")
		if err := os.WriteFile(targetPath, []byte("old"), 0o700); err != nil {
			t.Fatalf("seed target: %v", err)
		}
		if err := replaceBinary(targetPath, []byte("new"), 0o755); err != nil {
			t.Fatalf("replaceBinary: %v", err)
		}
		payload, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read target: %v", err)
		}
		if string(payload) != "new" {
			t.Fatalf("expected new payload, got %q", string(payload))
		}
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("stat target: %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("expected existing mode preserved, got %o", info.Mode().Perm())
		}
	})

	t.Run("new target uses provided mode", func(t *testing.T) {
		targetPath := filepath.Join(t.TempDir(), "bb")
		if err := replaceBinary(targetPath, []byte("new"), 0o755); err != nil {
			t.Fatalf("replaceBinary: %v", err)
		}
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("stat target: %v", err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("expected provided mode, got %o", info.Mode().Perm())
		}
	})
}
