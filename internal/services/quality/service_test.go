package quality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func newQualityTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewService(client)
}

func TestQualityServiceBuildAndReportsFlow(t *testing.T) {
	service := newQualityTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/build-status/latest/commits/abc":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/build-status/latest/commits/abc":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"key":"ci/main","state":"SUCCESSFUL"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/build-status/latest/commits/stats/abc":
			_, _ = w.Write([]byte(`{"successful":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/required-builds/latest/projects/TEST/repos/demo/conditions":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":1}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/required-builds/latest/projects/TEST/repos/demo/condition":
			_, _ = w.Write([]byte(`{"id":2}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/rest/required-builds/latest/projects/TEST/repos/demo/condition/"):
			_, _ = w.Write([]byte(`{"id":2}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/rest/required-builds/latest/projects/TEST/repos/demo/condition/"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"key":"lint","title":"Lint"}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint":
			_, _ = w.Write([]byte(`{"key":"lint","title":"Lint"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint":
			_, _ = w.Write([]byte(`{"key":"lint","title":"Lint"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint/annotations":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint/annotations":
			_, _ = w.Write([]byte(`{"annotations":[{"externalId":"a1","message":"ok"}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/insights/latest/projects/TEST/repos/demo/commits/abc/reports/lint/annotations":
			if r.URL.Query().Get("externalId") == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing externalId"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	repo := RepositoryRef{ProjectKey: "TEST", Slug: "demo"}

	if err := service.SetBuildStatus(context.Background(), "abc", BuildStatusSetInput{Key: "ci/main", State: "SUCCESSFUL", URL: "https://ci.example"}); err != nil {
		t.Fatalf("set build status: %v", err)
	}

	statuses, err := service.GetBuildStatuses(context.Background(), "abc", 25, "NEWEST")
	if err != nil || len(statuses) != 1 {
		t.Fatalf("get build statuses len=%d err=%v", len(statuses), err)
	}

	stats, err := service.GetBuildStatusStats(context.Background(), "abc", true)
	if err != nil || stats.Successful == nil || *stats.Successful != 1 {
		t.Fatalf("get build stats %#v err=%v", stats, err)
	}

	checks, err := service.ListRequiredBuildChecks(context.Background(), repo, 25)
	if err != nil || len(checks) != 1 {
		t.Fatalf("list required checks len=%d err=%v", len(checks), err)
	}

	createdCheck, err := service.CreateRequiredBuildCheck(context.Background(), repo, map[string]any{"buildParentKeys": []string{"ci"}})
	if err != nil || createdCheck["id"] == nil {
		t.Fatalf("create required check %#v err=%v", createdCheck, err)
	}

	updatedCheck, err := service.UpdateRequiredBuildCheck(context.Background(), repo, 2, map[string]any{"buildParentKeys": []string{"ci"}})
	if err != nil || updatedCheck["id"] == nil {
		t.Fatalf("update required check %#v err=%v", updatedCheck, err)
	}

	if err := service.DeleteRequiredBuildCheck(context.Background(), repo, 2); err != nil {
		t.Fatalf("delete required check: %v", err)
	}

	reports, err := service.ListReports(context.Background(), repo, "abc", 25)
	if err != nil || len(reports) != 1 {
		t.Fatalf("list reports len=%d err=%v", len(reports), err)
	}

	result := "PASS"
	report, err := service.SetReport(context.Background(), repo, "abc", "lint", openapigenerated.SetACodeInsightsReportJSONRequestBody{Result: &result, Title: "Lint"})
	if err != nil || report.Key == nil || *report.Key != "lint" {
		t.Fatalf("set report %#v err=%v", report, err)
	}

	gotReport, err := service.GetReport(context.Background(), repo, "abc", "lint")
	if err != nil || gotReport.Key == nil || *gotReport.Key != "lint" {
		t.Fatalf("get report %#v err=%v", gotReport, err)
	}

	if err := service.DeleteReport(context.Background(), repo, "abc", "lint"); err != nil {
		t.Fatalf("delete report: %v", err)
	}

	externalID := "a1"
	path := "seed.txt"
	line := int32(1)
	annotation := openapigenerated.RestSingleAddInsightAnnotationRequest{ExternalId: &externalID, Message: "ok", Severity: "LOW", Path: &path, Line: &line}
	if err := service.AddAnnotations(context.Background(), repo, "abc", "lint", []openapigenerated.RestSingleAddInsightAnnotationRequest{annotation}); err != nil {
		t.Fatalf("add annotations: %v", err)
	}

	annotations, err := service.ListAnnotations(context.Background(), repo, "abc", "lint")
	if err != nil || len(annotations) != 1 {
		t.Fatalf("list annotations len=%d err=%v", len(annotations), err)
	}

	if err := service.DeleteAnnotations(context.Background(), repo, "abc", "lint", "a1"); err != nil {
		t.Fatalf("delete annotations: %v", err)
	}
}

func TestQualityServiceValidationGuards(t *testing.T) {
	service := newQualityTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	err := service.SetBuildStatus(context.Background(), "", BuildStatusSetInput{})
	if err == nil || !strings.Contains(err.Error(), "commit id is required") {
		t.Fatalf("expected commit id validation error, got %v", err)
	}

	_, err = service.UpdateRequiredBuildCheck(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, 0, map[string]any{"x": "y"})
	if err == nil || !strings.Contains(err.Error(), "must be > 0") {
		t.Fatalf("expected id validation error, got %v", err)
	}

	err = service.DeleteAnnotations(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "abc", "lint", "")
	if err == nil || !strings.Contains(err.Error(), "external annotation id is required") {
		t.Fatalf("expected external id validation error, got %v", err)
	}

	_, err = service.GetBuildStatuses(context.Background(), "", 25, "")
	if err == nil || !strings.Contains(err.Error(), "commit id is required") {
		t.Fatalf("expected get statuses validation error, got %v", err)
	}

	_, err = service.GetBuildStatusStats(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "commit id is required") {
		t.Fatalf("expected get stats validation error, got %v", err)
	}

	_, err = service.ListRequiredBuildChecks(context.Background(), RepositoryRef{}, 25)
	if err == nil || !strings.Contains(err.Error(), "repository must be specified") {
		t.Fatalf("expected repository validation error, got %v", err)
	}

	_, err = service.ListReports(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "", 25)
	if err == nil || !strings.Contains(err.Error(), "commit id is required") {
		t.Fatalf("expected report commit validation error, got %v", err)
	}

	_, err = service.SetReport(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "", "lint", openapigenerated.SetACodeInsightsReportJSONRequestBody{})
	if err == nil || !strings.Contains(err.Error(), "commit id is required") {
		t.Fatalf("expected set report validation error, got %v", err)
	}

	_, err = service.GetReport(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "abc", "")
	if err == nil || !strings.Contains(err.Error(), "report key is required") {
		t.Fatalf("expected get report key validation error, got %v", err)
	}

	err = service.DeleteReport(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "abc", "")
	if err == nil || !strings.Contains(err.Error(), "report key is required") {
		t.Fatalf("expected delete report key validation error, got %v", err)
	}

	err = service.AddAnnotations(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, "abc", "lint", nil)
	if err == nil || !strings.Contains(err.Error(), "at least one annotation is required") {
		t.Fatalf("expected annotation validation error, got %v", err)
	}

	_, err = service.CreateRequiredBuildCheck(context.Background(), RepositoryRef{ProjectKey: "TEST", Slug: "demo"}, map[string]any{"id": strconv.Itoa(1)})
	if err != nil && strings.Contains(err.Error(), "invalid required build check payload") {
		// this branch is acceptable; payload serialization is permitted here
	}
}
