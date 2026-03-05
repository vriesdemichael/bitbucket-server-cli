package network

import (
	"fmt"
	"net/http"
	"os"
)

// SafeTransport is a http.RoundTripper that can be configured to block
// outgoing requests to non-local addresses, typically used during tests.
type SafeTransport struct {
	Base http.RoundTripper
}

func (t *SafeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if os.Getenv("BBSC_BLOCK_EXTERNAL_NETWORK") == "1" {
		host := req.URL.Hostname()
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			return nil, fmt.Errorf("external network access is disabled during tests (attempted to reach %s)", host)
		}
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewSafeClient returns an http.Client configured with SafeTransport.
func NewSafeClient(timeout string) *http.Client {
	return &http.Client{
		Transport: &SafeTransport{},
	}
}
