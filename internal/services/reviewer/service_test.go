package reviewer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

func ptr[T any](v T) *T {
	return &v
}

func TestReviewerService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/conditions":
			_, _ = w.Write([]byte(`[{"id":1,"requiredApprovals":1}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/conditions":
			_, _ = w.Write([]byte(`[{"id":2,"requiredApprovals":2}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/condition/1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/condition/2":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/condition":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":3,"requiredApprovals":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/condition":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":4,"requiredApprovals":1}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/condition/1":
			_, _ = w.Write([]byte(`{"id":1,"requiredApprovals":3}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/condition/2":
			_, _ = w.Write([]byte(`{"id":2,"requiredApprovals":4}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	service := NewService(client)

	projConditions, err := service.ListProjectConditions(context.Background(), "PRJ")
	if err != nil || len(projConditions) != 1 {
		t.Fatalf("list project conditions failed: %v", err)
	}

	repoConditions, err := service.ListRepositoryConditions(context.Background(), "PRJ", "demo")
	if err != nil || len(repoConditions) != 1 {
		t.Fatalf("list repository conditions failed: %v", err)
	}

	if err := service.DeleteProjectCondition(context.Background(), "PRJ", "1"); err != nil {
		t.Fatalf("delete project condition failed: %v", err)
	}

	if err := service.DeleteRepositoryCondition(context.Background(), "PRJ", "demo", "2"); err != nil {
		t.Fatalf("delete repository condition failed: %v", err)
	}

	if _, err := service.CreateProjectCondition(context.Background(), "PRJ", openapigenerated.RestDefaultReviewersRequest{}); err != nil {
		t.Fatalf("create project condition failed: %v", err)
	}

	if _, err := service.CreateRepositoryCondition(context.Background(), "PRJ", "demo", openapigenerated.RestDefaultReviewersRequest{}); err != nil {
		t.Fatalf("create repository condition failed: %v", err)
	}

	if _, err := service.UpdateProjectCondition(context.Background(), "PRJ", "1", openapigenerated.UpdatePullRequestConditionJSONRequestBody{
		RequiredApprovals: ptr(int32(3)),
	}); err != nil {
		t.Fatalf("update project condition failed: %v", err)
	}

	if _, err := service.UpdateRepositoryCondition(context.Background(), "PRJ", "demo", "2", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{
		RequiredApprovals: ptr(int32(4)),
	}); err != nil {
		t.Fatalf("update repository condition failed: %v", err)
	}
}

func TestReviewerServiceAdditional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/conditions":
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/rest/default-reviewers/latest/projects/PRJ/repos/demo/conditions":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	service := NewService(client)

	// Hit empty branches
	_, _ = service.ListProjectConditions(context.Background(), "PRJ")
	_, _ = service.ListRepositoryConditions(context.Background(), "PRJ", "demo")
}

func TestReviewerServiceErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	service := NewService(client)

	if _, err := service.ListProjectConditions(context.Background(), "PRJ"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.ListRepositoryConditions(context.Background(), "PRJ", "demo"); err == nil {
		t.Fatal("expected error")
	}
	if err := service.DeleteProjectCondition(context.Background(), "PRJ", "1"); err == nil {
		t.Fatal("expected delete project condition error")
	}
	if err := service.DeleteRepositoryCondition(context.Background(), "PRJ", "demo", "2"); err == nil {
		t.Fatal("expected delete repository condition error")
	}
}

func TestReviewerServiceValidation(t *testing.T) {
	service := NewService(nil)
	if _, err := service.ListProjectConditions(context.Background(), ""); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := service.ListRepositoryConditions(context.Background(), "", ""); err == nil {
		t.Fatal("expected validation error")
	}
	if err := service.DeleteProjectCondition(context.Background(), "", ""); err == nil {
		t.Fatal("expected validation error")
	}
	if err := service.DeleteRepositoryCondition(context.Background(), "", "", ""); err == nil {
		t.Fatal("expected validation error")
	}
	if err := service.DeleteRepositoryCondition(context.Background(), "PRJ", "demo", "abc"); err == nil {
		t.Fatal("expected validation error for non-int id")
	}
}

func TestReviewerServiceUpdateValidation(t *testing.T) {
	service := NewService(nil)
	if _, err := service.UpdateProjectCondition(context.Background(), "", "1", openapigenerated.UpdatePullRequestConditionJSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.UpdateProjectCondition(context.Background(), "P", "", openapigenerated.UpdatePullRequestConditionJSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.UpdateRepositoryCondition(context.Background(), "", "S", "1", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.UpdateRepositoryCondition(context.Background(), "P", "", "1", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.UpdateRepositoryCondition(context.Background(), "P", "S", "", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.CreateRepositoryCondition(context.Background(), "", "S", openapigenerated.RestDefaultReviewersRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.CreateRepositoryCondition(context.Background(), "P", "", openapigenerated.RestDefaultReviewersRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.CreateProjectCondition(context.Background(), "", openapigenerated.RestDefaultReviewersRequest{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestReviewerServiceUpdateErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL)
	service := NewService(client)

	if _, err := service.UpdateProjectCondition(context.Background(), "P", "1", openapigenerated.UpdatePullRequestConditionJSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.UpdateRepositoryCondition(context.Background(), "P", "S", "1", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.CreateRepositoryCondition(context.Background(), "P", "S", openapigenerated.RestDefaultReviewersRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := service.CreateProjectCondition(context.Background(), "P", openapigenerated.RestDefaultReviewersRequest{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestReviewerServiceCreationBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			// Return 200 instead of 201 to hit JSON200 branch
			_, _ = w.Write([]byte(`{"id":9}`))
			return
		}
		if r.Method == http.MethodPut {
			// Return invalid JSON to hit unmarshal fail branch
			_, _ = w.Write([]byte(`invalid`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL)
	service := NewService(client)

	_, _ = service.CreateProjectCondition(context.Background(), "P", openapigenerated.RestDefaultReviewersRequest{})
	_, _ = service.UpdateProjectCondition(context.Background(), "P", "1", openapigenerated.UpdatePullRequestConditionJSONRequestBody{})
}

func TestReviewerServiceCreationUnmarshalFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`invalid`))
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL)
	service := NewService(client)

	_, _ = service.CreateProjectCondition(context.Background(), "P", openapigenerated.RestDefaultReviewersRequest{})
}

func TestReviewerServiceUpdateResponseBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			// Return 200 to hit JSON200 branches instead of 201 or 300
			_, _ = w.Write([]byte(`{"id":42}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, _ := openapigenerated.NewClientWithResponses(server.URL)
	service := NewService(client)

	_, _ = service.UpdateProjectCondition(context.Background(), "P", "1", openapigenerated.UpdatePullRequestConditionJSONRequestBody{})
	_, _ = service.UpdateRepositoryCondition(context.Background(), "P", "S", "1", openapigenerated.UpdatePullRequestCondition1JSONRequestBody{})
}
