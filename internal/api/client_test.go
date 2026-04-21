package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/config"
)

// newTestClient creates a Client pointing at the given test server URL.
func newTestClient(serverURL string) *Client {
	cfg := &config.Config{
		Host:  serverURL,
		Token: "test-pat-token",
	}
	return NewClient(cfg)
}

// TestNewClient verifies that NewClient initializes correctly.
func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		Host:  "https://jira.example.com",
		Token: "mytoken",
	}
	c := NewClient(cfg)

	if c.host != cfg.Host {
		t.Errorf("host = %q, want %q", c.host, cfg.Host)
	}
	wantBearer := "Bearer mytoken"
	if c.authHeader != wantBearer {
		t.Errorf("authHeader = %q, want %q", c.authHeader, wantBearer)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if c.Issues == nil || c.Sprints == nil || c.Boards == nil ||
		c.Projects == nil || c.Users == nil || c.Filters == nil || c.Fields == nil {
		t.Error("API group fields should be initialized")
	}
}

// TestGet_200 verifies a normal 200 response.
func TestGet_200(t *testing.T) {
	want := map[string]string{"key": "TEST-1"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	data, err := c.Get("/rest/api/2/issue/TEST-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty response body")
	}
}

// TestRedirectPreservesAuthorization ensures Bearer survives a 302.
func TestRedirectPreservesAuthorization(t *testing.T) {
	var sawAuth atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			http.Redirect(w, r, "/second", http.StatusFound)
			return
		}
		if r.URL.Path == "/second" {
			sawAuth.Store(r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := &config.Config{Host: ts.URL, Token: "pat-x"}
	c := NewClient(cfg)
	data, err := c.Get("/first")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := sawAuth.Load().(string); got != "Bearer pat-x" {
		t.Fatalf("Authorization after redirect = %q", got)
	}
	var body map[string]string
	if err := json.Unmarshal(data, &body); err != nil || body["name"] != "ok" {
		t.Fatalf("body = %s err %v", data, err)
	}
}

// TestGet_401 verifies that 401 maps to the correct APIError.
func TestGet_401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/rest/api/2/issue/TEST-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if len(apiErr.ErrorMessages) == 0 {
		t.Error("expected error message for 401")
	}
}

// TestGet_403 verifies that 403 maps to the correct APIError.
func TestGet_403(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/rest/api/2/issue/TEST-1")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

// TestGet_404 verifies that 404 maps to the correct APIError.
func TestGet_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/rest/api/2/issue/NOTEXIST-1")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

// TestGet_4xx_ParsesBody verifies that other 4xx errors parse errorMessages from the response body.
func TestGet_4xx_ParsesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"errorMessages":["field 'summary' is required"],"errors":{}}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/rest/api/2/issue")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if len(apiErr.ErrorMessages) == 0 || apiErr.ErrorMessages[0] != "field 'summary' is required" {
		t.Errorf("unexpected error messages: %v", apiErr.ErrorMessages)
	}
}

// TestRetry_429_RetriesWithRetryAfter verifies that 429 retries respect the Retry-After header.
func TestRetry_429_RetriesWithRetryAfter(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	c.httpClient.Timeout = 5 * time.Second

	data, err := c.Get("/rest/api/2/issue/TEST-1")
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty response")
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls (2 retries + 1 success), got %d", atomic.LoadInt32(&callCount))
	}
}

// TestRetry_429_ExhaustsRetries verifies that 429 returns an error after exceeding max retries.
func TestRetry_429_ExhaustsRetries(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	_, err := c.Get("/rest/api/2/issue/TEST-1")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
	}

	if atomic.LoadInt32(&callCount) != 4 {
		t.Errorf("expected 4 calls (1 initial + 3 retries), got %d", atomic.LoadInt32(&callCount))
	}
}

// TestRetry_5xx_ExponentialBackoff verifies that 5xx errors use exponential backoff retries.
func TestRetry_5xx_ExponentialBackoff(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	start := time.Now()
	data, err := c.Get("/rest/api/2/issue/TEST-1")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty response")
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&callCount))
	}
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s elapsed for backoff, got %v", elapsed)
	}
}

// TestRetry_5xx_ExhaustsRetries verifies that 5xx returns an error after exceeding max retries.
func TestRetry_5xx_ExhaustsRetries(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"errorMessages":["service unavailable"]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	_, err := c.Get("/rest/api/2/issue/TEST-1")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", apiErr.StatusCode)
	}

	if atomic.LoadInt32(&callCount) != 4 {
		t.Errorf("expected 4 calls (1 initial + 3 retries), got %d", atomic.LoadInt32(&callCount))
	}
}

// TestPost_SendsBody verifies that POST correctly sends the request body.
func TestPost_SendsBody(t *testing.T) {
	var received map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"10001","key":"TEST-2"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	payload := map[string]string{"summary": "Test issue"}
	data, err := c.Post("/rest/api/2/issue", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty response")
	}
	if received["summary"] != "Test issue" {
		t.Errorf("received body = %v, want summary=Test issue", received)
	}
}

// TestDelete_204 verifies that DELETE correctly handles 204 No Content.
func TestDelete_204(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Delete("/rest/api/2/issue/TEST-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRequestHeaders verifies JSON GET omits Content-Type; Bearer and Accept/User-Agent are set.
func TestRequestHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("Authorization header missing")
		}
		if r.Header.Get("Authorization") != "Bearer test-pat-token" {
			t.Errorf("Authorization = %q, want Bearer test-pat-token", r.Header.Get("Authorization"))
		}
		if r.Method == http.MethodGet && r.Header.Get("Content-Type") != "" {
			t.Errorf("GET should not send Content-Type, got %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header missing")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/rest/api/2/myself")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRequestHeadersPOST verifies POST sets Content-Type application/json.
func TestRequestHeadersPOST(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Post("/rest/api/2/issue", map[string]string{"fields": "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestAPIError_Error verifies the APIError.Error() format.
func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with message",
			err:  &APIError{StatusCode: 404, ErrorMessages: []string{"resource not found"}},
			want: "Jira API error 404: resource not found",
		},
		{
			name: "without message",
			err:  &APIError{StatusCode: 500},
			want: "Jira API error 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
