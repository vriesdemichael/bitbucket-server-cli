package openapi

import (
	"net/http"
	"testing"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func TestMapStatusError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     []byte
		wantKind apperrors.Kind
	}{
		{
			name:     "200 OK",
			status:   http.StatusOK,
			body:     nil,
			wantKind: "",
		},
		{
			name:     "400 Bad Request",
			status:   http.StatusBadRequest,
			body:     []byte("invalid input"),
			wantKind: apperrors.KindValidation,
		},
		{
			name:     "401 Unauthorized",
			status:   http.StatusUnauthorized,
			body:     nil,
			wantKind: apperrors.KindAuthentication,
		},
		{
			name:     "403 Forbidden",
			status:   http.StatusForbidden,
			body:     nil,
			wantKind: apperrors.KindAuthorization,
		},
		{
			name:     "404 Not Found",
			status:   http.StatusNotFound,
			body:     nil,
			wantKind: apperrors.KindNotFound,
		},
		{
			name:     "409 Conflict",
			status:   http.StatusConflict,
			body:     nil,
			wantKind: apperrors.KindConflict,
		},
		{
			name:     "429 Too Many Requests",
			status:   http.StatusTooManyRequests,
			body:     nil,
			wantKind: apperrors.KindTransient,
		},
		{
			name:     "500 Internal Server Error",
			status:   http.StatusInternalServerError,
			body:     nil,
			wantKind: apperrors.KindTransient,
		},
		{
			name:     "503 Service Unavailable",
			status:   http.StatusServiceUnavailable,
			body:     nil,
			wantKind: apperrors.KindTransient,
		},
		{
			name:     "418 I'm a teapot (Other 4xx)",
			status:   http.StatusTeapot,
			body:     nil,
			wantKind: apperrors.KindPermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapStatusError(tt.status, tt.body)
			if tt.wantKind == "" {
				if err != nil {
					t.Errorf("MapStatusError() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Errorf("MapStatusError() error = nil, want kind %v", tt.wantKind)
				return
			}

			appErr, ok := err.(*apperrors.AppError)
			if !ok {
				t.Errorf("MapStatusError() error = %T, want *apperrors.AppError", err)
				return
			}

			if appErr.Kind != tt.wantKind {
				t.Errorf("MapStatusError() kind = %v, want %v", appErr.Kind, tt.wantKind)
			}
		})
	}
}
