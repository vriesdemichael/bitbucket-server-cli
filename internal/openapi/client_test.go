package openapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
)

type retryRoundTripperFunc func(*http.Request) (*http.Response, error)

func (function retryRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRetryTransport(t *testing.T) {
	t.Run("retries transient status", func(t *testing.T) {
		var attempts atomic.Int32
		transport := &retryTransport{
			base: retryRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				attempt := attempts.Add(1)
				status := http.StatusServiceUnavailable
				if attempt >= 3 {
					status = http.StatusOK
				}
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader("{}")),
					Header:     make(http.Header),
				}, nil
			}),
			retries:     2,
			baseBackoff: time.Nanosecond,
		}

		request, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected final 200 response, got %d", response.StatusCode)
		}
		if attempts.Load() != 3 {
			t.Fatalf("expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("retries transport error", func(t *testing.T) {
		var attempts atomic.Int32
		transport := &retryTransport{
			base: retryRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				attempt := attempts.Add(1)
				if attempt < 3 {
					return nil, errors.New("temporary failure")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
			}),
			retries:     2,
			baseBackoff: time.Nanosecond,
		}

		request, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected final 200 response, got %d", response.StatusCode)
		}
		if attempts.Load() != 3 {
			t.Fatalf("expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("returns last response when body cannot be replayed", func(t *testing.T) {
		var attempts atomic.Int32
		transport := &retryTransport{
			base: retryRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				attempts.Add(1)
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader("{}")),
					Header:     make(http.Header),
				}, nil
			}),
			retries:     1,
			baseBackoff: time.Nanosecond,
		}

		request, err := http.NewRequest(http.MethodPost, "http://example.test", io.NopCloser(strings.NewReader("payload")))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 response, got %d", response.StatusCode)
		}
		if attempts.Load() != 1 {
			t.Fatalf("expected one attempt due non-replayable body, got %d", attempts.Load())
		}
	})

	t.Run("returns terminal transport error after replay failure", func(t *testing.T) {
		var attempts atomic.Int32
		transport := &retryTransport{
			base: retryRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				attempt := attempts.Add(1)
				if attempt == 1 {
					return nil, errors.New("network down")
				}
				return nil, errors.New("unexpected")
			}),
			retries:     1,
			baseBackoff: time.Nanosecond,
		}

		request, err := http.NewRequest(http.MethodPost, "http://example.test", io.NopCloser(strings.NewReader("payload")))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		_, err = transport.RoundTrip(request)
		if err == nil {
			t.Fatal("expected transport error")
		}
		if !strings.Contains(err.Error(), "network down") {
			t.Fatalf("expected first error to be returned, got: %v", err)
		}
	})

	t.Run("returns last response when request get body fails", func(t *testing.T) {
		transport := &retryTransport{
			base: retryRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader("{}")),
					Header:     make(http.Header),
				}, nil
			}),
			retries:     1,
			baseBackoff: time.Nanosecond,
		}

		request, err := http.NewRequest(http.MethodPost, "http://example.test", strings.NewReader("payload"))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		request.GetBody = func() (io.ReadCloser, error) {
			return nil, errors.New("cannot clone body")
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected last 503 response, got %d", response.StatusCode)
		}
	})

	t.Run("falls back to default transport when base is nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("{}"))
		}))
		defer server.Close()

		transport := &retryTransport{retries: 0, baseBackoff: time.Nanosecond}
		request, err := http.NewRequest(http.MethodGet, server.URL, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 response, got %d", response.StatusCode)
		}
	})
}

func TestNewClientWithResponsesFromConfigInvalidCA(t *testing.T) {
	_, err := NewClientWithResponsesFromConfig(config.AppConfig{
		BitbucketURL:   "http://localhost:7990",
		CAFile:         "/definitely/missing/ca.pem",
		RequestTimeout: time.Second,
		RetryCount:     1,
		RetryBackoff:   time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected transport initialization error")
	}
}
