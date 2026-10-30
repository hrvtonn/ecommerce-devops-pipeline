package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/metrics"
)

// The router is exercised without services: every case below is answered
// before a service would be called.
func newTestRouter() http.Handler {
	return NewRouter(config.Config{Env: "test"}, Services{}, metrics.New())
}

func do(t *testing.T, h http.Handler, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var decoded map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("invalid JSON response: %v", err)
		}
	}
	return rec, decoded
}

func TestHealth(t *testing.T) {
	rec, body := do(t, newTestRouter(), http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Errorf("got %d %v", rec.Code, body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected security headers")
	}
}

func TestNotFound(t *testing.T) {
	rec, body := do(t, newTestRouter(), http.MethodGet, "/nope?x=1", "")
	if rec.Code != http.StatusNotFound || body["code"] != "ROUTE_NOT_FOUND" || body["message"] != "Route GET /nope?x=1 not found" {
		t.Errorf("got %d %v", rec.Code, body)
	}
}

func TestValidationErrors(t *testing.T) {
	h := newTestRouter()

	t.Run("invalid body", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/api/v1/products", `{"name": "A"}`)
		if rec.Code != http.StatusBadRequest || body["code"] != "VALIDATION_ERROR" {
			t.Errorf("got %d %v", rec.Code, body)
		}
		if details, _ := body["details"].([]any); len(details) == 0 {
			t.Error("expected validation details")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		rec, body := do(t, h, http.MethodPost, "/api/v1/categories", `{"name":`)
		if rec.Code != http.StatusBadRequest || body["code"] != "INVALID_JSON" {
			t.Errorf("got %d %v", rec.Code, body)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		rec, body := do(t, h, http.MethodGet, "/api/v1/orders/abc", "")
		if rec.Code != http.StatusBadRequest || body["code"] != "VALIDATION_ERROR" {
			t.Errorf("got %d %v", rec.Code, body)
		}
	})
}

func TestDocsAndMetrics(t *testing.T) {
	h := newTestRouter()

	rec, body := do(t, h, http.MethodGet, "/api-docs.json", "")
	if rec.Code != http.StatusOK || body["openapi"] != "3.0.3" {
		t.Errorf("api-docs.json: got %d", rec.Code)
	}

	rec, _ = do(t, h, http.MethodGet, "/metrics", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `http_requests_total{method="GET",route="/api-docs.json",status_code="200"} 1`) {
		t.Errorf("metrics: got %d\n%s", rec.Code, rec.Body.String())
	}
}

func TestRateLimiter(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := newRateLimiter(time.Minute, 2, "/health").Middleware(ok)

	for i, want := range []int{200, 200, 429} {
		rec, _ := do(t, h, http.MethodGet, "/api/v1/products", "")
		if rec.Code != want {
			t.Errorf("request %d: got %d, want %d", i+1, rec.Code, want)
		}
	}
	if rec, _ := do(t, h, http.MethodGet, "/health", ""); rec.Code != http.StatusOK {
		t.Errorf("/health must not be rate limited, got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/products", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("got %d %v", rec.Code, rec.Header())
	}
}
