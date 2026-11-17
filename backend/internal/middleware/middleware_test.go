package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPIKeyAuth(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"missing key", "", http.StatusUnauthorized},
		{"wrong key", "nope", http.StatusUnauthorized},
		{"valid key", "secret", http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("X-API-Key", tc.header)
			}
			if got := do(h, req).Code; got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAPIKeyAuth_DisabledWhenEmpty(t *testing.T) {
	h := APIKeyAuth("")(okHandler)
	if got := do(h, httptest.NewRequest(http.MethodGet, "/", nil)).Code; got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
}

func TestHeaderTokenAuth_CustomHeader(t *testing.T) {
	h := HeaderTokenAuth("X-Internal-Token", "tok")(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Internal-Token", "tok")
	if got := do(h, req).Code; got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := do(SecurityHeaders(okHandler), httptest.NewRequest(http.MethodGet, "/", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestRateLimit(t *testing.T) {
	h := RateLimit(2)(okHandler)

	newReq := func(addr string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = addr
		return req
	}

	for i := 1; i <= 2; i++ {
		if got := do(h, newReq("10.0.0.1:1234")).Code; got != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, got)
		}
	}
	rec := do(h, newReq("10.0.0.1:9999")) // mismo host, otro puerto
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
	// Otro cliente no se ve afectado.
	if got := do(h, newReq("10.0.0.2:1234")).Code; got != http.StatusOK {
		t.Fatalf("other client status = %d, want 200", got)
	}
}

func TestRateLimit_DisabledWhenZero(t *testing.T) {
	h := RateLimit(0)(okHandler)
	for i := 0; i < 5; i++ {
		if got := do(h, httptest.NewRequest(http.MethodGet, "/", nil)).Code; got != http.StatusOK {
			t.Fatalf("status = %d, want 200", got)
		}
	}
}

func TestClientKey_WithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "weird-address"
	if got := clientKey(req); got != "weird-address" {
		t.Fatalf("clientKey = %q", got)
	}
}

func TestRequestLogger_DoesNotLogQueryString(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := RequestLogger(log)(okHandler)

	do(h, httptest.NewRequest(http.MethodGet, "/api/v1/customers/lookup?email=ana@example.com", nil))

	out := buf.String()
	if !strings.Contains(out, `"path":"/api/v1/customers/lookup"`) {
		t.Fatalf("log missing path: %s", out)
	}
	if strings.Contains(out, "ana@example.com") {
		t.Fatalf("log leaked PII: %s", out)
	}
}
