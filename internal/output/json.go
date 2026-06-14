package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const SchemaVersion = "1.0"

// jsonMarshalIndent is json.MarshalIndent; overridden in tests for error-path coverage.
var jsonMarshalIndent = json.MarshalIndent

// jsonMarshal is json.Marshal; overridden in tests for error-path coverage.
var jsonMarshal = json.Marshal

// CompactJSON controls whether JSON output is compact or indented.
var CompactJSON bool

// EnvelopeJSON controls whether success payloads are wrapped in the agent envelope.
// Raw format disables this while keeping JSON error envelopes.
var EnvelopeJSON = true

// CommandStartTime is set by the CLI root command for duration metadata.
var CommandStartTime time.Time

type Envelope struct {
	OK            bool             `json:"ok"`
	SchemaVersion string           `json:"schema_version"`
	Data          any              `json:"data,omitempty"`
	Error         *EnvelopeError   `json:"error,omitempty"`
	Meta          map[string]int64 `json:"meta,omitempty"`
}

type EnvelopeError struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
}

func marshalForOutput(v any) ([]byte, error) {
	if CompactJSON {
		return jsonMarshal(v)
	}
	return jsonMarshalIndent(v, "", "  ")
}

func durationMeta() map[string]int64 {
	duration := int64(0)
	if !CommandStartTime.IsZero() {
		duration = time.Since(CommandStartTime).Milliseconds()
	}
	return map[string]int64{"duration_ms": duration}
}

func successEnvelope(v any) Envelope {
	return Envelope{
		OK:            true,
		SchemaVersion: SchemaVersion,
		Data:          v,
		Meta:          durationMeta(),
	}
}

// PrintJSON outputs v as JSON to stdout.
func PrintJSON(v any) {
	if EnvelopeJSON {
		v = successEnvelope(v)
	}
	data, err := marshalForOutput(v)
	if err != nil {
		PrintErrorJSONWithCode("json marshal error: "+err.Error(), 0, ErrUnknown)
		return
	}
	fmt.Println(string(data))
}

// ErrorCode classifies errors for machine consumption.
type ErrorCode string

const (
	ErrConfig          ErrorCode = "E_CONFIG"
	ErrAuth            ErrorCode = "E_AUTH"
	ErrForbidden       ErrorCode = "E_FORBIDDEN"
	ErrNotFound        ErrorCode = "E_NOT_FOUND"
	ErrRateLimit       ErrorCode = "E_RATE_LIMITED"
	ErrServer          ErrorCode = "E_SERVER"
	ErrValidation      ErrorCode = "E_VALIDATION"
	ErrNetwork         ErrorCode = "E_NETWORK"
	ErrConfirmRequired ErrorCode = "E_CONFIRMATION_REQUIRED"
	ErrConflict        ErrorCode = "E_CONFLICT"
	ErrTimeout         ErrorCode = "E_TIMEOUT"
	ErrUnknown         ErrorCode = "E_UNKNOWN"
)

// ErrorCodeFromStatus maps HTTP status codes to error codes.
func ErrorCodeFromStatus(statusCode int) ErrorCode {
	switch statusCode {
	case 408:
		return ErrTimeout
	case 401:
		return ErrAuth
	case 403:
		return ErrForbidden
	case 404:
		return ErrNotFound
	case 409:
		return ErrConflict
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

func RetryableForErrorCode(code ErrorCode) bool {
	switch code {
	case ErrRateLimit, ErrServer, ErrNetwork, ErrTimeout:
		return true
	default:
		return false
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
	case ErrConfirmRequired:
		return "Run the same write command with --dry-run, then retry with --confirm <token>"
	case ErrConflict:
		return "Re-run --dry-run and retry with the new confirm token"
	case ErrTimeout:
		return "Retry with backoff; increase timeout if the command supports it"
	default:
		return ""
	}
}

// PrintErrorJSON outputs an error envelope as JSON to stdout.
func PrintErrorJSON(msg string, statusCode int) {
	code := ErrorCodeFromStatus(statusCode)
	if statusCode == 0 {
		code = ErrUnknown
	}
	PrintErrorJSONWithCode(msg, statusCode, code)
}

// PrintErrorJSONWithCode outputs an error envelope with an explicit error code.
func PrintErrorJSONWithCode(msg string, statusCode int, code ErrorCode) {
	details := map[string]any{}
	if statusCode != 0 {
		details["status_code"] = statusCode
	}
	if hint := HintForErrorCode(code); hint != "" {
		details["hint"] = hint
	}
	if len(details) == 0 {
		details = nil
	}
	payload := Envelope{
		OK:            false,
		SchemaVersion: SchemaVersion,
		Meta:          durationMeta(),
		Error: &EnvelopeError{
			Code:      code,
			Message:   msg,
			Details:   details,
			Retryable: RetryableForErrorCode(code),
		},
	}
	data, err := marshalForOutput(payload)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, `{"ok":false,"schema_version":%q,"error":{"code":%q,"message":%q,"retryable":%v},"meta":{"duration_ms":%d}}`+"\n",
			SchemaVersion, code, msg, RetryableForErrorCode(code), durationMeta()["duration_ms"])
		return
	}
	fmt.Println(string(data))
}
