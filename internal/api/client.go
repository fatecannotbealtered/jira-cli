package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/config"
)

const defaultMaxRetries = 3

// defaultUserAgent returns JIRA_CLI_USER_AGENT if set, otherwise a clear CLI UA.
func defaultUserAgent() string {
	if ua := strings.TrimSpace(os.Getenv("JIRA_CLI_USER_AGENT")); ua != "" {
		return ua
	}
	return "jira-cli"
}

func maxRetries() int {
	if s := strings.TrimSpace(os.Getenv("JIRA_CLI_MAX_RETRIES")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return n
		}
	}
	return defaultMaxRetries
}

func retryBaseWait() time.Duration {
	if s := strings.TrimSpace(os.Getenv("JIRA_CLI_RETRY_BASE_MS")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Second
}

func retryAfterWait(h http.Header) time.Duration {
	if ra := h.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return retryBaseWait()
}

var (
	uploadOpenFile       = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	uploadCreateFormFile = func(w *multipart.Writer, fieldname, filename string) (io.Writer, error) {
		return w.CreateFormFile(fieldname, filename)
	}
)

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Go may drop Authorization on redirect; re-apply so Bearer survives SSO hops.
			if len(via) > 0 {
				if auth := via[len(via)-1].Header.Get("Authorization"); auth != "" {
					req.Header.Set("Authorization", auth)
				}
				if ua := via[len(via)-1].Header.Get("User-Agent"); ua != "" {
					req.Header.Set("User-Agent", ua)
				}
			}
			return nil
		},
	}
}

func (c *Client) applyJSONHeaders(req *http.Request, jsonBody bool) {
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent())
	if jsonBody {
		req.Header.Set("Content-Type", "application/json")
	}
}

// APIError represents an error returned by the Jira API.
type APIError struct {
	StatusCode    int
	ErrorMessages []string
	Errors        map[string]string
}

func (e *APIError) Error() string {
	if len(e.ErrorMessages) > 0 {
		return fmt.Sprintf("Jira API error %d: %s", e.StatusCode, e.ErrorMessages[0])
	}
	if len(e.Errors) > 0 {
		return fmt.Sprintf("Jira API error %d: %v", e.StatusCode, e.Errors)
	}
	return fmt.Sprintf("Jira API error %d", e.StatusCode)
}

// apiErrorBody is used to parse Jira error response bodies.
type apiErrorBody struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// Client wraps the Jira REST API v2 HTTP client.
type Client struct {
	host       string
	authHeader string // Always "Bearer <PAT>"
	httpClient *http.Client

	// API groups
	Issues   *IssueAPI
	Sprints  *SprintAPI
	Boards   *BoardAPI
	Projects *ProjectAPI
	Users    *UserAPI
	Filters  *FilterAPI
	Fields   *FieldAPI
}

func (c *Client) restPath(resource string) string {
	return "/rest/api/2" + resource
}

// NewClient creates a new API client from the given configuration.
func NewClient(cfg *config.Config) *Client {
	c := &Client{
		host:       cfg.Host,
		authHeader: "Bearer " + strings.TrimSpace(cfg.Token),
		httpClient: newHTTPClient(),
	}

	c.Issues = &IssueAPI{client: c}
	c.Sprints = &SprintAPI{client: c}
	c.Boards = &BoardAPI{client: c}
	c.Projects = &ProjectAPI{client: c}
	c.Users = &UserAPI{client: c}
	c.Filters = &FilterAPI{client: c}
	c.Fields = &FieldAPI{client: c}

	return c
}

// doWithRetry executes a request with retry logic (429 Retry-After, 5xx exponential backoff).
func (c *Client) doWithRetry(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	for attempt := 0; ; attempt++ {
		var reqBody io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, 0, fmt.Errorf("encoding request body: %w", err)
			}
			reqBody = bytes.NewReader(encoded)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.host+path, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}

		c.applyJSONHeaders(req, body != nil)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("executing request: %w", err)
		}

		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", readErr)
		}

		statusCode := resp.StatusCode

		// Success
		if statusCode < 400 {
			return data, statusCode, nil
		}

		// HTTP 429: rate limited, read Retry-After
		if statusCode == http.StatusTooManyRequests {
			if attempt >= maxRetries() {
				return nil, statusCode, c.parseError(statusCode, data)
			}
			wait := retryAfterWait(resp.Header)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			}
			continue
		}

		// HTTP 5xx: exponential backoff
		if statusCode >= 500 {
			if attempt >= maxRetries() {
				return nil, statusCode, c.parseError(statusCode, data)
			}
			wait := time.Duration(1<<uint(attempt)) * retryBaseWait()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			}
			continue
		}

		// Other 4xx: do not retry
		return nil, statusCode, c.parseError(statusCode, data)
	}
}

// parseError converts an HTTP status code and response body into an APIError.
func (c *Client) parseError(statusCode int, data []byte) *APIError {
	var original apiErrorBody
	if len(data) > 0 {
		_ = json.Unmarshal(data, &original)
	}

	switch statusCode {
	case http.StatusUnauthorized:
		msgs := []string{"not logged in: run 'jira-cli login' or set JIRA_TOKEN env var"}
		if len(original.ErrorMessages) > 0 {
			msgs = append(msgs, original.ErrorMessages...)
		}
		return &APIError{StatusCode: 401, ErrorMessages: msgs, Errors: original.Errors}
	case http.StatusForbidden:
		msgs := []string{"permission denied: check your PAT scope"}
		if len(original.ErrorMessages) > 0 {
			msgs = append(msgs, original.ErrorMessages...)
		}
		return &APIError{StatusCode: 403, ErrorMessages: msgs, Errors: original.Errors}
	case http.StatusNotFound:
		msgs := []string{"resource not found"}
		if len(original.ErrorMessages) > 0 {
			msgs = append(msgs, original.ErrorMessages...)
		}
		return &APIError{StatusCode: 404, ErrorMessages: msgs, Errors: original.Errors}
	default:
		apiErr := &APIError{StatusCode: statusCode}
		if len(original.ErrorMessages) > 0 {
			apiErr.ErrorMessages = original.ErrorMessages
		}
		if len(original.Errors) > 0 {
			apiErr.Errors = original.Errors
		}
		if len(apiErr.ErrorMessages) == 0 && len(apiErr.Errors) == 0 {
			apiErr.ErrorMessages = []string{fmt.Sprintf("unexpected status code %d", statusCode)}
		}
		return apiErr
	}
}

// Get sends a GET request.
func (c *Client) Get(path string) ([]byte, error) {
	data, _, err := c.doWithRetry(context.Background(), http.MethodGet, path, nil)
	return data, err
}

// GetWithContext sends a GET request with context.
func (c *Client) GetWithContext(ctx context.Context, path string) ([]byte, error) {
	data, _, err := c.doWithRetry(ctx, http.MethodGet, path, nil)
	return data, err
}

// Post sends a POST request.
func (c *Client) Post(path string, body any) ([]byte, error) {
	data, _, err := c.doWithRetry(context.Background(), http.MethodPost, path, body)
	return data, err
}

// PostWithContext sends a POST request with context.
func (c *Client) PostWithContext(ctx context.Context, path string, body any) ([]byte, error) {
	data, _, err := c.doWithRetry(ctx, http.MethodPost, path, body)
	return data, err
}

// Put sends a PUT request.
func (c *Client) Put(path string, body any) ([]byte, error) {
	data, _, err := c.doWithRetry(context.Background(), http.MethodPut, path, body)
	return data, err
}

// Delete sends a DELETE request.
func (c *Client) Delete(path string) error {
	_, _, err := c.doWithRetry(context.Background(), http.MethodDelete, path, nil)
	return err
}

// Upload uploads a file attachment using multipart/form-data.
func (c *Client) Upload(ctx context.Context, path, filePath string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		f, err := uploadOpenFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("opening file %s: %w", filePath, err)
		}

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)

		part, err := uploadCreateFormFile(w, "file", filepath.Base(filePath))
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("creating form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("copying file content: %w", err)
		}
		_ = f.Close()
		_ = w.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+path, &buf)
		if err != nil {
			return nil, fmt.Errorf("creating upload request: %w", err)
		}

		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", defaultUserAgent())
		req.Header.Set("X-Atlassian-Token", "no-check")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing upload request: %w", err)
		}

		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading upload response: %w", readErr)
		}

		statusCode := resp.StatusCode
		if statusCode < 400 {
			return data, nil
		}

		if statusCode == http.StatusTooManyRequests {
			if attempt >= maxRetries() {
				return nil, c.parseError(statusCode, data)
			}
			wait := retryAfterWait(resp.Header)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		if statusCode >= 500 {
			if attempt >= maxRetries() {
				return nil, c.parseError(statusCode, data)
			}
			wait := time.Duration(1<<uint(attempt)) * retryBaseWait()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		return nil, c.parseError(statusCode, data)
	}
}

// Download streams a resource to dst. rawURL may be an absolute URL or a host
// path ("/..."); to prevent SSRF, an absolute URL must point at the configured
// Jira host. Uses Bearer auth like every other request.
func (c *Client) Download(ctx context.Context, rawURL, dst string) error {
	target := rawURL
	switch {
	case strings.HasPrefix(rawURL, "/"):
		target = c.host + rawURL
	case strings.HasPrefix(rawURL, c.host+"/"), rawURL == c.host:
		// same host, allowed as-is
	default:
		return fmt.Errorf("refusing to download from URL outside %s", c.host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", defaultUserAgent())

	// Large attachments (e.g. screen recordings) can exceed the shared 30s
	// client timeout; download with no overall deadline and rely on ctx.
	dl := &http.Client{CheckRedirect: c.httpClient.CheckRedirect}
	resp, err := dl.Do(req)
	if err != nil {
		return fmt.Errorf("executing download request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return c.parseError(resp.StatusCode, data)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dst, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", dst, err)
	}
	return nil
}

// ===== API group types (methods implemented in their own files) =====

// IssueAPI wraps Issue-related API calls.
type IssueAPI struct{ client *Client }

// SprintAPI wraps Sprint-related API calls.
type SprintAPI struct{ client *Client }

// BoardAPI wraps Board-related API calls.
type BoardAPI struct{ client *Client }

// ProjectAPI wraps Project-related API calls.
type ProjectAPI struct{ client *Client }

// UserAPI wraps User-related API calls.
type UserAPI struct{ client *Client }

// FilterAPI wraps Filter-related API calls.
type FilterAPI struct{ client *Client }

// FieldAPI wraps Field-related API calls.
type FieldAPI struct{ client *Client }
