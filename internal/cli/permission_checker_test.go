package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	pullrequestservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/pullrequest"
)

func newPermissionCheckerTestClient(t *testing.T, handler http.HandlerFunc) (*PermissionChecker, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := openapigenerated.NewClientWithResponses(server.URL + "/rest")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	return NewPermissionChecker(client), server
}

func TestPermissionCheckerCheckRepoPermission(t *testing.T) {
	t.Run("caches successful repo permission probe", func(t *testing.T) {
		var calls atomic.Int32
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/latest/repos" {
				http.NotFound(w, r)
				return
			}
			if r.URL.Query().Get("projectkey") != "PRJ" || r.URL.Query().Get("name") != "" || r.URL.Query().Get("permission") != "REPO_ADMIN" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("bad query"))
				return
			}
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"values":[{"slug":"demo","name":"Repository Display Name","project":{"key":"PRJ"}}],"isLastPage":true}`))
		})

		if err := checker.CheckRepoPermission(t.Context(), "PRJ", "demo", openapigenerated.REPOADMIN); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if err := checker.CheckRepoPermission(t.Context(), "PRJ", "demo", openapigenerated.REPOADMIN); err != nil {
			t.Fatalf("expected cached success, got: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected one HTTP call, got %d", calls.Load())
		}
	})

	t.Run("returns authorization error when repo list is empty", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"values":[],"isLastPage":true}`))
		})

		err := checker.CheckRepoPermission(t.Context(), "PRJ", "demo", openapigenerated.REPOWRITE)
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})

	t.Run("returns authorization error when list does not include requested slug", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"values":[{"slug":"other","project":{"key":"PRJ"}}],"isLastPage":true}`))
		})

		err := checker.CheckRepoPermission(t.Context(), "PRJ", "demo", openapigenerated.REPOWRITE)
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})

	t.Run("maps HTTP status errors", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		})

		err := checker.CheckRepoPermission(t.Context(), "PRJ", "demo", openapigenerated.REPOREAD)
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})
}

func TestPermissionCheckerCheckProjectWrite(t *testing.T) {
	t.Run("returns success when project lookup matches permission-filtered result", func(t *testing.T) {
		var projectCalls atomic.Int32
		var listCalls atomic.Int32
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/rest/api/latest/projects/PRJ":
				projectCalls.Add(1)
				w.Header().Set("Content-Type", "application/json;charset=UTF-8")
				_, _ = w.Write([]byte(`{"key":"PRJ","name":"Project"}`))
			case "/rest/api/latest/projects":
				listCalls.Add(1)
				if r.URL.Query().Get("name") != "Project" || r.URL.Query().Get("permission") != "PROJECT_WRITE" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("bad query"))
					return
				}
				w.Header().Set("Content-Type", "application/json;charset=UTF-8")
				_, _ = w.Write([]byte(`{"values":[{"key":"PRJ","name":"Project"}],"isLastPage":true}`))
			default:
				http.NotFound(w, r)
			}
		})

		if err := checker.CheckProjectWrite(t.Context(), "PRJ"); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if err := checker.CheckProjectWrite(t.Context(), "PRJ"); err != nil {
			t.Fatalf("expected cached success, got: %v", err)
		}
		if projectCalls.Load() != 1 || listCalls.Load() != 1 {
			t.Fatalf("expected one project call and one list call, got %d and %d", projectCalls.Load(), listCalls.Load())
		}
	})

	t.Run("returns internal error when project name is missing", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/latest/projects/PRJ" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"key":"PRJ"}`))
		})

		err := checker.CheckProjectWrite(t.Context(), "PRJ")
		if !apperrors.IsKind(err, apperrors.KindInternal) {
			t.Fatalf("expected internal error, got: %v", err)
		}
	})

	t.Run("returns authorization error when filtered list omits project", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/rest/api/latest/projects/PRJ":
				w.Header().Set("Content-Type", "application/json;charset=UTF-8")
				_, _ = w.Write([]byte(`{"key":"PRJ","name":"Project"}`))
			case "/rest/api/latest/projects":
				w.Header().Set("Content-Type", "application/json;charset=UTF-8")
				_, _ = w.Write([]byte(`{"values":[{"key":"OTHER","name":"Project"}],"isLastPage":true}`))
			default:
				http.NotFound(w, r)
			}
		})

		err := checker.CheckProjectWrite(t.Context(), "PRJ")
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})

	t.Run("maps project lookup status errors", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("missing"))
		})

		err := checker.CheckProjectWrite(t.Context(), "PRJ")
		if !apperrors.IsKind(err, apperrors.KindNotFound) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})
}

func TestPermissionCheckerCheckProjectAdmin(t *testing.T) {
	t.Run("caches successful admin probe", func(t *testing.T) {
		var calls atomic.Int32
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Path != "/rest/api/latest/projects/PRJ/permissions/users" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"values":[{"user":{"name":"alice"}}],"isLastPage":true}`))
		})

		if err := checker.CheckProjectAdmin(t.Context(), "PRJ"); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if err := checker.CheckProjectAdmin(t.Context(), "PRJ"); err != nil {
			t.Fatalf("expected cached success, got: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected one HTTP call, got %d", calls.Load())
		}
	})

	t.Run("maps status errors", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		})

		err := checker.CheckProjectAdmin(t.Context(), "PRJ")
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})
}

func TestPermissionCheckerCheckProjectCreate(t *testing.T) {
	t.Run("treats bad request as authorized and caches result", func(t *testing.T) {
		var calls atomic.Int32
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/latest/projects" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("name is required"))
		})

		if err := checker.CheckProjectCreate(t.Context()); err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if err := checker.CheckProjectCreate(t.Context()); err != nil {
			t.Fatalf("expected cached success, got: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected one HTTP call, got %d", calls.Load())
		}
	})

	t.Run("maps forbidden as authorization error", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		})

		err := checker.CheckProjectCreate(t.Context())
		if !apperrors.IsKind(err, apperrors.KindAuthorization) {
			t.Fatalf("expected authorization error, got: %v", err)
		}
	})

	t.Run("returns permanent error for unexpected status", func(t *testing.T) {
		checker, _ := newPermissionCheckerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":"PRJ"}`))
		})

		err := checker.CheckProjectCreate(t.Context())
		if !apperrors.IsKind(err, apperrors.KindPermanent) {
			t.Fatalf("expected permanent error, got: %v", err)
		}
	})
}

func TestCommentOwnedByUser(t *testing.T) {
	username := "alice"
	commentWithName := openapigenerated.RestComment{Author: &struct {
		Active       *bool                                   `json:"active,omitempty"`
		AvatarUrl    *string                                 `json:"avatarUrl,omitempty"`
		DisplayName  *string                                 `json:"displayName,omitempty"`
		EmailAddress *string                                 `json:"emailAddress,omitempty"`
		Id           *int32                                  `json:"id,omitempty"`
		Links        *map[string]interface{}                 `json:"links,omitempty"`
		Name         *string                                 `json:"name,omitempty"`
		Slug         *string                                 `json:"slug,omitempty"`
		Type         *openapigenerated.RestCommentAuthorType `json:"type,omitempty"`
	}{Name: &username}}
	if !commentOwnedByUser(commentWithName, " alice ") {
		t.Fatal("expected comment ownership match by name")
	}

	slug := "alice"
	commentWithSlug := openapigenerated.RestComment{Author: &struct {
		Active       *bool                                   `json:"active,omitempty"`
		AvatarUrl    *string                                 `json:"avatarUrl,omitempty"`
		DisplayName  *string                                 `json:"displayName,omitempty"`
		EmailAddress *string                                 `json:"emailAddress,omitempty"`
		Id           *int32                                  `json:"id,omitempty"`
		Links        *map[string]interface{}                 `json:"links,omitempty"`
		Name         *string                                 `json:"name,omitempty"`
		Slug         *string                                 `json:"slug,omitempty"`
		Type         *openapigenerated.RestCommentAuthorType `json:"type,omitempty"`
	}{Slug: &slug}}
	if !commentOwnedByUser(commentWithSlug, "alice") {
		t.Fatal("expected comment ownership match by slug")
	}

	if commentOwnedByUser(openapigenerated.RestComment{}, "alice") {
		t.Fatal("expected missing author to fail ownership check")
	}
	if commentOwnedByUser(commentWithName, "") {
		t.Fatal("expected blank username to fail ownership check")
	}
	if commentOwnedByUser(commentWithName, "bob") {
		t.Fatal("expected mismatched username to fail ownership check")
	}
}

func TestReviewerApprovedByUser(t *testing.T) {
	reviewers := []pullrequestservice.Reviewer{
		{Name: "alice", Status: "UNAPPROVED", Approved: false},
		{Name: "bob", Status: "APPROVED", Approved: false},
		{Name: "carol", Status: "UNAPPROVED", Approved: true},
	}

	if !reviewerApprovedByUser(reviewers, " bob ") {
		t.Fatal("expected approved reviewer status match")
	}
	if !reviewerApprovedByUser(reviewers, "carol") {
		t.Fatal("expected approved reviewer flag match")
	}
	if reviewerApprovedByUser(reviewers, "alice") {
		t.Fatal("expected unapproved reviewer to fail")
	}
	if reviewerApprovedByUser(reviewers, "") {
		t.Fatal("expected blank username to fail")
	}
}

func TestRootOptionsPermissionCheckerFor(t *testing.T) {
	clientA := &openapigenerated.ClientWithResponses{}
	clientB := &openapigenerated.ClientWithResponses{}

	var nilOptions *rootOptions
	if checker := nilOptions.permissionCheckerFor(clientA); checker != nil {
		t.Fatalf("expected nil options to return nil checker, got %#v", checker)
	}

	options := &rootOptions{}
	if checker := options.permissionCheckerFor(nil); checker != nil {
		t.Fatalf("expected nil client to return nil checker, got %#v", checker)
	}

	checkerA := options.permissionCheckerFor(clientA)
	if checkerA == nil {
		t.Fatal("expected checker to be created")
	}
	checkerB := options.permissionCheckerFor(clientB)
	if checkerA != checkerB {
		t.Fatal("expected checker to be reused once created")
	}
	if checkerA.client != clientA {
		t.Fatalf("expected first client to be retained, got %p want %p", checkerA.client, clientA)
	}
}

func TestLoadQualityRepoServiceAndClientReturnsSelectorValidationError(t *testing.T) {
	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://example.local")
	t.Setenv("BITBUCKET_PROJECT_KEY", "PRJ")
	t.Setenv("BITBUCKET_REPO_SLUG", "repo")

	_, _, _, err := loadQualityRepoServiceAndClient("bad-selector")
	if !apperrors.IsKind(err, apperrors.KindValidation) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestLoadConfigAndClientPropagatesConfigValidationError(t *testing.T) {
	t.Setenv("BB_DISABLE_STORED_CONFIG", "1")
	t.Setenv("BITBUCKET_URL", "http://example.local")
	t.Setenv("BB_CA_FILE", "/definitely/missing-ca.pem")

	_, _, err := loadConfigAndClient()
	if !apperrors.IsKind(err, apperrors.KindValidation) {
		t.Fatalf("expected validation error, got: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "BB_CA_FILE is invalid") {
		t.Fatalf("expected config validation message, got: %v", err)
	}
}
