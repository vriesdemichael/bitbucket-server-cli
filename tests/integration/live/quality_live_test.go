//go:build live

package live_test

import (
	"context"
	"fmt"
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
	dataTitle := "coverage"
	dataType := "NUMBER"
	value := map[string]any{"value": 100}

	reportRequest := openapigenerated.SetACodeInsightsReportJSONRequestBody{
		Title:  title,
		Result: &result,
		Data: []openapigenerated.RestInsightReportData{
			{Title: &dataTitle, Type: &dataType, Value: &value},
		},
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
}
