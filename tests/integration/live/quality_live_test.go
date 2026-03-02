//go:build live

package live_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
)

func TestLiveBuildStatusSetAndGet(t *testing.T) {
	harness := newLiveHarness(t)
	service := qualityservice.NewService(harness.client)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 2)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	commitID := repo.CommitIDs[0]
	buildKey := fmt.Sprintf("live-build-%d", time.Now().UnixNano()%100000)

	err = service.SetBuildStatus(ctx, commitID, qualityservice.BuildStatusSetInput{
		Key:   buildKey,
		State: "SUCCESSFUL",
		URL:   "https://example.invalid/live-build",
		Name:  "Live Build",
	})
	if err != nil {
		t.Fatalf("set build status failed: %v", err)
	}

	statuses, err := service.GetBuildStatuses(ctx, commitID, 25, "NEWEST")
	if err != nil {
		t.Fatalf("get build statuses failed: %v", err)
	}

	found := false
	for _, status := range statuses {
		if status.Key != nil && *status.Key == buildKey {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected build status key=%s in response", buildKey)
	}
}

func TestLiveCodeInsightsReportSetAndGet(t *testing.T) {
	harness := newLiveHarness(t)
	service := qualityservice.NewService(harness.client)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := seeded.Repos[0]
	commitID := repo.CommitIDs[0]
	reportKey := fmt.Sprintf("live-report-%d", time.Now().UnixNano()%100000)
	title := "Live Insights"
	result := "PASS"
	reportRequest := openapigenerated.SetACodeInsightsReportJSONRequestBody{
		Title:  title,
		Result: &result,
	}

	_, err = service.SetReport(
		ctx,
		qualityservice.RepositoryRef{ProjectKey: seeded.Key, Slug: repo.Slug},
		commitID,
		reportKey,
		reportRequest,
	)
	if err != nil {
		t.Fatalf("set report failed: %v", err)
	}

	report, err := service.GetReport(
		ctx,
		qualityservice.RepositoryRef{ProjectKey: seeded.Key, Slug: repo.Slug},
		commitID,
		reportKey,
	)
	if err != nil {
		t.Fatalf("get report failed: %v", err)
	}

	if report.Key == nil || *report.Key != reportKey {
		t.Fatalf("expected report key=%s, got %#v", reportKey, report.Key)
	}
	if report.Title == nil || *report.Title != title {
		t.Fatalf("expected report title=%s, got %#v", title, report.Title)
	}

	if err := service.DeleteReport(
		ctx,
		qualityservice.RepositoryRef{ProjectKey: seeded.Key, Slug: repo.Slug},
		commitID,
		reportKey,
	); err != nil {
		t.Fatalf("delete report failed: %v", err)
	}
}

func TestLiveRequiredBuildCheckLifecycle(t *testing.T) {
	harness := newLiveHarness(t)
	service := qualityservice.NewService(harness.client)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := qualityservice.RepositoryRef{ProjectKey: seeded.Key, Slug: seeded.Repos[0].Slug}
	payload := map[string]any{
		"buildParentKeys": []string{"ci"},
		"refMatcher": map[string]any{
			"id": "refs/heads/master",
		},
	}

	created, err := service.CreateRequiredBuildCheck(ctx, repo, payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "returned 500") {
			t.Skipf("required build check endpoint returned server error in live environment: %v", err)
		}
		t.Fatalf("create required build check failed: %v", err)
	}

	checkID, ok := requiredBuildCheckID(created)
	if !ok || checkID <= 0 {
		t.Fatalf("expected created check id, got %#v", created)
	}

	if _, err := service.UpdateRequiredBuildCheck(ctx, repo, checkID, payload); err != nil {
		t.Fatalf("update required build check failed: %v", err)
	}

	checks, err := service.ListRequiredBuildChecks(ctx, repo, 25)
	if err != nil {
		t.Fatalf("list required build checks failed: %v", err)
	}
	if len(checks) == 0 {
		t.Fatalf("expected at least one required build check")
	}

	if err := service.DeleteRequiredBuildCheck(ctx, repo, checkID); err != nil {
		t.Fatalf("delete required build check failed: %v", err)
	}
}

func TestLiveCodeInsightsAnnotationsLifecycle(t *testing.T) {
	harness := newLiveHarness(t)
	service := qualityservice.NewService(harness.client)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	seeded, err := harness.seedProjectWithRepositories(ctx, 1, 1)
	if err != nil {
		t.Fatalf("seed project with repositories failed: %v", err)
	}

	repo := qualityservice.RepositoryRef{ProjectKey: seeded.Key, Slug: seeded.Repos[0].Slug}
	commitID := seeded.Repos[0].CommitIDs[0]
	reportKey := fmt.Sprintf("live-report-annotations-%d", time.Now().UnixNano()%100000)

	result := "PASS"
	title := "Live Annotations"
	_, err = service.SetReport(ctx, repo, commitID, reportKey, openapigenerated.SetACodeInsightsReportJSONRequestBody{Title: title, Result: &result})
	if err != nil {
		t.Fatalf("set report for annotations failed: %v", err)
	}

	externalID := fmt.Sprintf("ann-%d", time.Now().UnixNano()%100000)
	path := "seed.txt"
	line := int32(1)
	annotations := []openapigenerated.RestSingleAddInsightAnnotationRequest{{
		ExternalId: &externalID,
		Message:    "integration annotation",
		Severity:   "LOW",
		Path:       &path,
		Line:       &line,
	}}

	if err := service.AddAnnotations(ctx, repo, commitID, reportKey, annotations); err != nil {
		t.Fatalf("add annotations failed: %v", err)
	}

	listed, err := service.ListAnnotations(ctx, repo, commitID, reportKey)
	if err != nil {
		t.Fatalf("list annotations failed: %v", err)
	}
	if len(listed) == 0 {
		t.Fatalf("expected at least one annotation")
	}

	if err := service.DeleteAnnotations(ctx, repo, commitID, reportKey, externalID); err != nil {
		t.Fatalf("delete annotations failed: %v", err)
	}

	if err := service.DeleteReport(ctx, repo, commitID, reportKey); err != nil {
		t.Fatalf("delete report failed: %v", err)
	}
}

func requiredBuildCheckID(payload map[string]any) (int64, bool) {
	value, ok := payload["id"]
	if !ok {
		return 0, false
	}

	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}
