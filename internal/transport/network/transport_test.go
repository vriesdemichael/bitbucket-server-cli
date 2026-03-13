package network

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeTransport(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		blockEnv  string
		wantError bool
	}{
		{
			name:      "localhost allowed when blocked",
			url:       "http://localhost:8080",
			blockEnv:  "1",
			wantError: false,
		},
		{
			name:      "127.0.0.1 allowed when blocked",
			url:       "http://127.0.0.1:8080",
			blockEnv:  "1",
			wantError: false,
		},
		{
			name:      "::1 allowed when blocked",
			url:       "http://[::1]:8080",
			blockEnv:  "1",
			wantError: false,
		},
		{
			name:      "external blocked when enabled",
			url:       "http://example.com",
			blockEnv:  "1",
			wantError: true,
		},
		{
			name:      "external allowed when disabled",
			url:       "http://example.com",
			blockEnv:  "0",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("BB_BLOCK_EXTERNAL_NETWORK", tt.blockEnv)

			// Use a dummy transport for the success cases to avoid real network calls
			// if the URL is actually reachable.
			base := &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200}, nil
				},
			}

			transport := &SafeTransport{Base: base}
			req, _ := http.NewRequest(http.MethodGet, tt.url, nil)

			_, err := transport.RoundTrip(req)
			if (err != nil) != tt.wantError {
				t.Errorf("SafeTransport.RoundTrip() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

type mockTransport struct {
	roundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestNewSafeClient(t *testing.T) {
	client := NewSafeClient("20s")
	if client == nil {
		t.Fatal("expected client to be initialized")
	}
	if _, ok := client.Transport.(*SafeTransport); !ok {
		t.Errorf("expected SafeTransport, got %T", client.Transport)
	}
}

func TestNewSafeTransport(t *testing.T) {
	t.Run("insecure mode enabled", func(t *testing.T) {
		roundTripper, err := NewSafeTransport(TLSOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		safe, ok := roundTripper.(*SafeTransport)
		if !ok {
			t.Fatalf("expected SafeTransport, got %T", roundTripper)
		}

		base, ok := safe.Base.(*http.Transport)
		if !ok {
			t.Fatalf("expected *http.Transport base, got %T", safe.Base)
		}

		if base.TLSClientConfig == nil || !base.TLSClientConfig.InsecureSkipVerify {
			t.Fatal("expected InsecureSkipVerify to be true")
		}
	})

	t.Run("missing ca file", func(t *testing.T) {
		_, err := NewSafeTransport(TLSOptions{CAFile: filepath.Join(t.TempDir(), "missing.pem")})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid ca file", func(t *testing.T) {
		caFile := filepath.Join(t.TempDir(), "bad.pem")
		if err := os.WriteFile(caFile, []byte("not-a-cert"), 0o600); err != nil {
			t.Fatalf("write ca file: %v", err)
		}

		_, err := NewSafeTransport(TLSOptions{CAFile: caFile})
		if err == nil {
			t.Fatal("expected parse error")
		}
	})
}
