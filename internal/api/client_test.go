package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, fmt.Errorf("read failed") }
func (errReadCloser) Close() error             { return nil }

func init() {
	_ = os.Setenv("JIRA_CLI_RETRY_BASE_MS", "0")
}

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

	data, err := c.Get("/rest/api/2/issue/TEST-1")

	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty response")
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&callCount))
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

func TestAPIError_ErrorWithErrorsMap(t *testing.T) {
	err := &APIError{StatusCode: 400, Errors: map[string]string{"summary": "required"}}
	got := err.Error()
	if !strings.Contains(got, "400") || !strings.Contains(got, "summary") {
		t.Errorf("Error() = %q", got)
	}
}

func TestDefaultUserAgent(t *testing.T) {
	var seenUA atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	t.Setenv("JIRA_CLI_USER_AGENT", "custom-agent")
	if _, err := c.Get("/ua-custom"); err != nil {
		t.Fatalf("custom UA: %v", err)
	}
	if got := seenUA.Load().(string); got != "custom-agent" {
		t.Errorf("User-Agent = %q, want custom-agent", got)
	}

	t.Setenv("JIRA_CLI_USER_AGENT", "")
	if _, err := c.Get("/ua-default"); err != nil {
		t.Fatalf("default UA: %v", err)
	}
	if got := seenUA.Load().(string); got != "jira-cli" {
		t.Errorf("User-Agent = %q, want jira-cli", got)
	}

	t.Setenv("JIRA_CLI_USER_AGENT", "   ")
	if _, err := c.Get("/ua-trimmed"); err != nil {
		t.Fatalf("whitespace UA: %v", err)
	}
	if got := seenUA.Load().(string); got != "jira-cli" {
		t.Errorf("User-Agent = %q, want jira-cli for blank env", got)
	}
}

func TestGetWithContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	data, err := c.GetWithContext(context.Background(), "/rest/api/2/myself")
	if err != nil {
		t.Fatalf("GetWithContext: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected response body")
	}
}

func TestPostWithContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"id":"1"}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	data, err := c.PostWithContext(context.Background(), "/rest/api/2/issue", map[string]string{"summary": "x"})
	if err != nil {
		t.Fatalf("PostWithContext: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected response body")
	}
}

func TestParseError_WithOriginalMessages(t *testing.T) {
	c := newTestClient("http://example.com")
	body := []byte(`{"errorMessages":["server detail"],"errors":{"field":"bad"}}`)

	for _, tc := range []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "not logged in"},
		{http.StatusForbidden, "permission denied"},
		{http.StatusNotFound, "resource not found"},
	} {
		apiErr := c.parseError(tc.code, body)
		if apiErr.StatusCode != tc.code {
			t.Fatalf("code %d: StatusCode = %d", tc.code, apiErr.StatusCode)
		}
		if len(apiErr.ErrorMessages) < 2 || apiErr.ErrorMessages[1] != "server detail" {
			t.Errorf("code %d: messages = %v", tc.code, apiErr.ErrorMessages)
		}
		if apiErr.Errors["field"] != "bad" {
			t.Errorf("code %d: errors = %v", tc.code, apiErr.Errors)
		}
	}
}

func TestParseError_DefaultBranches(t *testing.T) {
	c := newTestClient("http://example.com")

	withErrors := c.parseError(418, []byte(`{"errors":{"tea":"short"}}`))
	if withErrors.ErrorMessages != nil {
		t.Errorf("expected nil ErrorMessages, got %v", withErrors.ErrorMessages)
	}
	if withErrors.Errors["tea"] != "short" {
		t.Errorf("errors = %v", withErrors.Errors)
	}

	empty := c.parseError(418, nil)
	if len(empty.ErrorMessages) != 1 || empty.ErrorMessages[0] != "unexpected status code 418" {
		t.Errorf("empty body error = %v", empty.ErrorMessages)
	}
}

func TestDoWithRetry_MarshalError(t *testing.T) {
	c := newTestClient("http://example.com")
	_, _, err := c.doWithRetry(context.Background(), http.MethodPost, "/x", make(chan int))
	if err == nil || !strings.Contains(err.Error(), "encoding request body") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestDoWithRetry_InvalidURL(t *testing.T) {
	c := newTestClient("http://example.com")
	c.host = "://bad-host"
	_, _, err := c.doWithRetry(context.Background(), http.MethodGet, "/x", nil)
	if err == nil || !strings.Contains(err.Error(), "creating request") {
		t.Fatalf("expected request creation error, got %v", err)
	}
}

func TestDoWithRetry_NetworkError(t *testing.T) {
	c := newTestClient("http://example.com")
	c.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network down")
		}),
	}
	_, _, err := c.doWithRetry(context.Background(), http.MethodGet, "/x", nil)
	if err == nil || !strings.Contains(err.Error(), "executing request") {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestDoWithRetry_ReadBodyError(t *testing.T) {
	c := newTestClient("http://example.com")
	c.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}
	_, code, err := c.doWithRetry(context.Background(), http.MethodGet, "/x", nil)
	if err == nil || !strings.Contains(err.Error(), "reading response body") {
		t.Fatalf("expected read error, got code=%d err=%v", code, err)
	}
}

func TestDoWithRetry_429ContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.doWithRetry(ctx, http.MethodGet, "/rest/api/2/issue", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoWithRetry_429ContextCanceledDuringWait(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, _, err := c.doWithRetry(ctx, http.MethodGet, "/rest/api/2/issue", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled during wait, got %v", err)
	}
}

func TestDoWithRetry_5xxContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.doWithRetry(ctx, http.MethodGet, "/rest/api/2/issue", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoWithRetry_5xxContextCanceledDuringWait(t *testing.T) {
	t.Setenv("JIRA_CLI_RETRY_BASE_MS", "100")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, _, err := c.doWithRetry(ctx, http.MethodGet, "/rest/api/2/issue", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled during wait, got %v", err)
	}
}

func TestDoWithRetry_429PositiveRetryAfter(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	start := time.Now()
	if _, err := c.Get("/rest/api/2/issue"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("expected ~1s Retry-After wait, got %v", elapsed)
	}
}

func TestDoWithRetry_429InvalidRetryAfter(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "not-a-number")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	if _, err := c.Get("/rest/api/2/issue"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestRedirectTooManyRedirects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Get("/loop")
	if err == nil || !strings.Contains(err.Error(), "executing request") {
		t.Fatalf("expected redirect error, got %v", err)
	}
}

func TestUpload_Success(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Errorf("X-Atlassian-Token = %q", r.Header.Get("X-Atlassian-Token"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[{"id":"1"}]`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	data, err := c.Upload(context.Background(), "/rest/api/2/issue/TEST-1/attachments", filePath)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected upload response")
	}
}

func TestUpload_ContextCanceled(t *testing.T) {
	c := newTestClient("http://example.com")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Upload(ctx, "/upload", filepath.Join(t.TempDir(), "missing.txt"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestUpload_FileOpenError(t *testing.T) {
	c := newTestClient("http://example.com")
	_, err := c.Upload(context.Background(), "/upload", filepath.Join(t.TempDir(), "no-such-file.txt"))
	if err == nil || !strings.Contains(err.Error(), "opening file") {
		t.Fatalf("expected open error, got %v", err)
	}
}

func TestUpload_NetworkError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient("http://example.com")
	c.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("upload network down")
		}),
	}
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if err == nil || !strings.Contains(err.Error(), "executing upload request") {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestUpload_ReadResponseError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient("http://example.com")
	c.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if err == nil || !strings.Contains(err.Error(), "reading upload response") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestUpload_429RetryAndExhaust(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	if _, err := c.Upload(context.Background(), "/upload", filePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&callCount))
	}

	atomic.StoreInt32(&callCount, 0)
	ts429 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts429.Close()

	c429 := newTestClient(ts429.URL)
	_, err := c429.Upload(context.Background(), "/upload", filePath)
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 429 {
		t.Fatalf("expected 429 APIError, got %v", err)
	}
	if atomic.LoadInt32(&callCount) != 4 {
		t.Errorf("expected 4 calls on exhaust, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestUpload_429ContextCanceledDuringWait(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := c.Upload(ctx, "/upload", filePath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestUpload_429PositiveRetryAfter(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	start := time.Now()
	if _, err := c.Upload(context.Background(), "/upload", filePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("expected ~1s Retry-After wait, got %v", elapsed)
	}
}

func TestUpload_5xxRetryAndExhaust(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	if _, err := c.Upload(context.Background(), "/upload", filePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	atomic.StoreInt32(&callCount, 0)
	ts503 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts503.Close()

	c503 := newTestClient(ts503.URL)
	_, err := c503.Upload(context.Background(), "/upload", filePath)
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 503 {
		t.Fatalf("expected 503 APIError, got %v", err)
	}
}

func TestUpload_5xxContextCanceledDuringWait(t *testing.T) {
	t.Setenv("JIRA_CLI_RETRY_BASE_MS", "100")
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := c.Upload(ctx, "/upload", filePath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestUpload_4xxError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"errorMessages":["bad upload"]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 400 {
		t.Fatalf("expected 400 APIError, got %v", err)
	}
}

func TestUpload_429InvalidRetryAfter(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "invalid")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	if _, err := c.Upload(context.Background(), "/upload", filePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpload_CopyError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	origOpen := uploadOpenFile
	uploadOpenFile = func(string) (io.ReadCloser, error) {
		return errReadCloser{}, nil
	}
	t.Cleanup(func() { uploadOpenFile = origOpen })

	c := newTestClient("http://example.com")
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if err == nil || !strings.Contains(err.Error(), "copying file content") {
		t.Fatalf("expected copy error, got %v", err)
	}
}

func TestUpload_CreateFormFileError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	origCreate := uploadCreateFormFile
	uploadCreateFormFile = func(*multipart.Writer, string, string) (io.Writer, error) {
		return nil, fmt.Errorf("create form file failed")
	}
	t.Cleanup(func() { uploadCreateFormFile = origCreate })

	c := newTestClient("http://example.com")
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if err == nil || !strings.Contains(err.Error(), "creating form file") {
		t.Fatalf("expected form file error, got %v", err)
	}
}

func TestUpload_InvalidHost(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "attach.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient("http://example.com")
	c.host = "://bad-host"
	_, err := c.Upload(context.Background(), "/upload", filePath)
	if err == nil || !strings.Contains(err.Error(), "creating upload request") {
		t.Fatalf("expected request creation error, got %v", err)
	}
}

func TestDownload_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-pat-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file-bytes"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	dst := filepath.Join(t.TempDir(), "out.bin")

	// path form
	if err := c.Download(context.Background(), "/secure/attachment/1/a.png", dst); err != nil {
		t.Fatalf("Download path form: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "file-bytes" {
		t.Errorf("content = %q", got)
	}

	// absolute same-host URL form
	if err := c.Download(context.Background(), ts.URL+"/secure/attachment/1/a.png", dst); err != nil {
		t.Fatalf("Download absolute form: %v", err)
	}
}

func TestDownload_SSRFGuard(t *testing.T) {
	c := newTestClient("https://jira.example.com")
	err := c.Download(context.Background(), "https://evil.example.com/x", filepath.Join(t.TempDir(), "x"))
	if err == nil || !strings.Contains(err.Error(), "refusing to download") {
		t.Fatalf("expected SSRF guard error, got %v", err)
	}
}

func TestDownload_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errorMessages":["gone"]}`)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	err := c.Download(context.Background(), "/x", filepath.Join(t.TempDir(), "x"))
	if apiErr, ok := err.(*APIError); !ok || apiErr.StatusCode != 404 {
		t.Fatalf("expected 404 APIError, got %v", err)
	}
}

func TestDownload_CreateFileError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	// dst dir does not exist -> os.Create fails
	err := c.Download(context.Background(), "/x", filepath.Join(t.TempDir(), "missing-dir", "x"))
	if err == nil || !strings.Contains(err.Error(), "creating file") {
		t.Fatalf("expected create file error, got %v", err)
	}
}
