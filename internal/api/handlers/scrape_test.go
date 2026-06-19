package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestScraper_BlocksPrivateIP verifies that validateURL rejects requests that
// would resolve to private or loopback addresses (SSRF protection).
func TestScraper_BlocksPrivateIP(t *testing.T) {
	t.Run("blocks private IP 192.168.x.x", func(t *testing.T) {
		err := validateURL("http://192.168.1.1/")
		if err == nil {
			t.Fatal("expected error for private IP 192.168.1.1, got nil")
		}
	})

	t.Run("blocks loopback 127.0.0.1", func(t *testing.T) {
		err := validateURL("http://127.0.0.1/")
		if err == nil {
			t.Fatal("expected error for loopback 127.0.0.1, got nil")
		}
	})

	t.Run("blocks hostname resolving to loopback via httptest server", func(t *testing.T) {
		// httptest.NewServer binds to 127.0.0.1 — its URL will be http://127.0.0.1:<port>
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		// ts.URL is http://127.0.0.1:<port> which resolves to a loopback address
		err := validateURL(ts.URL)
		if err == nil {
			t.Fatalf("expected error for loopback httptest server URL %s, got nil", ts.URL)
		}
	})

	t.Run("allows public IP 8.8.8.8", func(t *testing.T) {
		// 8.8.8.8 is a public IP; validateURL should not block it.
		// (DNS lookup will resolve it directly as an IP.)
		err := validateURL("http://8.8.8.8/")
		if err != nil {
			t.Fatalf("expected no error for public IP 8.8.8.8, got: %v", err)
		}
	})
}
