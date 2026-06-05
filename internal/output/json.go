package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// jsonMarshalIndent is json.MarshalIndent; overridden in tests for error-path coverage.
var jsonMarshalIndent = json.MarshalIndent

// jsonMarshal is json.Marshal; overridden in tests for error-path coverage.
var jsonMarshal = json.Marshal

// CompactJSON controls whether JSON output is compact or indented.
var CompactJSON bool

func marshalForOutput(v any) ([]byte, error) {
	if CompactJSON {
		return jsonMarshal(v)
	}
	return jsonMarshalIndent(v, "", "  ")
}

// PrintJSON outputs v as JSON to stdout.
func PrintJSON(v any) {
	data, err := marshalForOutput(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// ErrorCode classifies errors for machine consumption.
type ErrorCode string

const (
	ErrConfig     ErrorCode = "CONFIG_ERROR"
	ErrAuth       ErrorCode = "AUTH_REQUIRED"
	ErrForbidden  ErrorCode = "FORBIDDEN"
	ErrNotFound   ErrorCode = "NOT_FOUND"
	ErrRateLimit  ErrorCode = "RATE_LIMITED"
	ErrServer     ErrorCode = "SERVER_ERROR"
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrNetwork    ErrorCode = "NETWORK_ERROR"
	ErrUnknown    ErrorCode = "UNKNOWN_ERROR"
)

// ErrorCodeFromStatus maps HTTP status codes to error codes.
func ErrorCodeFromStatus(statusCode int) ErrorCode {
	switch statusCode {
	case 401:
		return ErrAuth
	case 403:
		return ErrForbidden
	case 404:
		return ErrNotFound
	case 429:
		return ErrRateLimit
	default:
		if statusCode >= 500 {
			return ErrServer
		}
		if statusCode >= 400 {
			return ErrValidation
		}
		return ErrUnknown
	}
}

// HintForErrorCode returns an actionable hint for the given error code.
func HintForErrorCode(code ErrorCode) string {
	switch code {
	case ErrConfig:
		return "Run 'jira-cli login' or set JIRA_HOST and JIRA_TOKEN environment variables"
	case ErrAuth:
		return "Check your PAT; regenerate in Jira DC Profile > Personal Access Tokens"
	case ErrForbidden:
		return "Check your PAT scope and project permissions"
	case ErrNotFound:
		return "Verify the resource key/ID exists and you have permission to view it"
	case ErrRateLimit:
		return "Wait and retry; reduce request frequency"
	case ErrServer:
		return "Jira server error; try again later"
	case ErrValidation:
		return "Check command arguments and flags"
	case ErrNetwork:
		return "Check host URL and network connectivity"
	default:
		return ""
	}
}

// PrintErrorJSON outputs an error message as JSON to stderr.
func PrintErrorJSON(msg string, statusCode int) {
	code := ErrorCodeFromStatus(statusCode)
	if statusCode == 0 {
		code = ErrUnknown
	}
	payload := struct {
		Error      string    `json:"error"`
		StatusCode int       `json:"statusCode"`
		ErrorCode  ErrorCode `json:"errorCode"`
		Hint       string    `json:"hint,omitempty"`
	}{
		Error:      msg,
		StatusCode: statusCode,
		ErrorCode:  code,
		Hint:       HintForErrorCode(code),
	}
	data, err := marshalForOutput(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": %q, "statusCode": %d, "errorCode": %q}`+"\n", msg, statusCode, code)
		return
	}
	fmt.Fprintln(os.Stderr, string(data))
}

// PrintErrorJSONWithCode outputs an error with an explicit error code.
func PrintErrorJSONWithCode(msg string, statusCode int, code ErrorCode) {
	payload := struct {
		Error      string    `json:"error"`
		StatusCode int       `json:"statusCode"`
		ErrorCode  ErrorCode `json:"errorCode"`
		Hint       string    `json:"hint,omitempty"`
	}{
		Error:      msg,
		StatusCode: statusCode,
		ErrorCode:  code,
		Hint:       HintForErrorCode(code),
	}
	data, err := marshalForOutput(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": %q, "statusCode": %d, "errorCode": %q}`+"\n", msg, statusCode, code)
		return
	}
	fmt.Fprintln(os.Stderr, string(data))
}
