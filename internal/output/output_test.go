package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// ─── runeWidth ────────────────────────────────────────────────────────────────

func TestRuneWidth_ASCII(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 5},
		{"PROJ-123", 8},
		{"a b c", 5},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := runeWidth(tc.input)
			if got != tc.want {
				t.Errorf("runeWidth(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestRuneWidth_CJK(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"中", 2},
		{"中文", 4},
		{"你好世界", 8},
		{"AB中文", 6}, // 2 ASCII + 2 CJK*2
		{"日本語", 6},
		{"한국어", 6},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := runeWidth(tc.input)
			if got != tc.want {
				t.Errorf("runeWidth(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// ─── stripAnsi ────────────────────────────────────────────────────────────────

func TestStripAnsi(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\033[31mred\033[0m", "red"},
		{"\033[1;36mbold cyan\033[0m", "bold cyan"},
		{"\033[90mgray text\033[0m", "gray text"},
		{"no \033[32mcolor\033[0m here", "no color here"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := stripAnsi(tc.input)
			if got != tc.want {
				t.Errorf("stripAnsi(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ─── truncate ─────────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	cases := []struct {
		input    string
		maxWidth int
		want     string
	}{
		{"hello", 10, "hello"},         // no truncation needed
		{"hello", 5, "hello"},          // exactly fits the width
		{"hello world", 8, "hello w…"}, // truncated with ellipsis
		{"hello", 3, "he…"},            // short truncation
		{"hello", 1, "…"},              // very short
		{"hello", 0, ""},               // zero width
		{"中文测试", 6, "中文…"},             // CJK truncation (中=2, 文=2, …=1, total=5, fits in 6)
		{"中文测试", 8, "中文测试"},            // exactly fits (4*2=8), no truncation
		{"中文测试", 10, "中文测试"},           // no truncation needed (4*2=8 <= 10)
	}
	for _, tc := range cases {
		name := fmt.Sprintf("%q/%d", tc.input, tc.maxWidth)
		t.Run(name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxWidth)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxWidth, got, tc.want)
			}
		})
	}
}

func TestTruncate_TruncatedWidthWithinBound(t *testing.T) {
	// The display width after truncation should not exceed maxWidth
	inputs := []string{"hello world", "中文测试内容", "ABCDE中文FGH", "short"}
	maxWidths := []int{3, 5, 7, 10, 15}
	for _, s := range inputs {
		for _, mw := range maxWidths {
			result := truncate(s, mw)
			w := runeWidth(result)
			if w > mw {
				t.Errorf("truncate(%q, %d) = %q (width %d) exceeds maxWidth", s, mw, result, w)
			}
		}
	}
}

// ─── PrintJSON ────────────────────────────────────────────────────────────────

// captureStdout captures output written to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestPrintJSON_ValidJSON(t *testing.T) {
	type payload struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}
	p := payload{Key: "PROJ-123", Value: 42}

	out := captureStdout(t, func() {
		PrintJSON(p)
	})

	// Output must be valid JSON
	var got payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("PrintJSON output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if got.Key != p.Key || got.Value != p.Value {
		t.Errorf("PrintJSON round-trip: got %+v, want %+v", got, p)
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	out := captureStdout(t, func() {
		PrintJSON(map[string]string{"hello": "world"})
	})
	// Should contain indentation (2 spaces)
	if !strings.Contains(out, "  ") {
		t.Errorf("PrintJSON output should be indented, got: %s", out)
	}
}

func TestPrintJSON_Map(t *testing.T) {
	data := map[string]any{
		"error":      "not found",
		"statusCode": 404,
	}
	out := captureStdout(t, func() {
		PrintJSON(data)
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("PrintJSON map output is not valid JSON: %v", err)
	}
	if got["error"] != "not found" {
		t.Errorf("expected error='not found', got %v", got["error"])
	}
}

// ─── StatusBadge / PriorityBadge ─────────────────────────────────────────────

func TestStatusBadge_ContainsStatus(t *testing.T) {
	statuses := []string{"To Do", "Open", "Backlog", "In Progress", "In Review", "Doing", "Done", "Closed", "Resolved", "Unknown"}
	for _, s := range statuses {
		result := StatusBadge(s)
		plain := stripAnsi(result)
		if plain != s {
			t.Errorf("StatusBadge(%q) plain text = %q, want %q", s, plain, s)
		}
	}
}

func TestPriorityBadge_ContainsPriority(t *testing.T) {
	priorities := []string{"Highest", "High", "Medium", "Low", "Lowest", "Unknown"}
	for _, p := range priorities {
		result := PriorityBadge(p)
		plain := stripAnsi(result)
		if plain != p {
			t.Errorf("PriorityBadge(%q) plain text = %q, want %q", p, plain, p)
		}
	}
}

// ─── isTerminal / colorize / print helpers ───────────────────────────────────

func TestIsTerminal(t *testing.T) {
	if isTerminal(os.Stdout) {
		t.Log("stdout is a terminal")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	_ = w.Close()
	if isTerminal(r) {
		t.Error("pipe reader should not be a terminal")
	}
	_ = r.Close()
	if isTerminal(r) {
		t.Error("closed pipe should not be a terminal")
	}
}

func TestColorize_NoColor(t *testing.T) {
	old := noColor
	noColor = true
	t.Cleanup(func() { noColor = old })

	got := colorize(ansiRed, "hello")
	if got != "hello" {
		t.Errorf("colorize with noColor=true = %q, want plain text", got)
	}
}

func TestColorize_WithColor(t *testing.T) {
	old := noColor
	noColor = false
	t.Cleanup(func() { noColor = old })

	got := colorize(ansiRed, "hello")
	if got != ansiRed+"hello"+ansiReset {
		t.Errorf("colorize with color enabled = %q", got)
	}
}

func TestFormatFunctions(t *testing.T) {
	old := noColor
	t.Run("no color", func(t *testing.T) {
		noColor = true
		t.Cleanup(func() { noColor = old })
		for _, fn := range []struct {
			name string
			got  string
			in   string
		}{
			{"FormatCyan", FormatCyan("x"), "x"},
			{"FormatCyanBold", FormatCyanBold("x"), "x"},
			{"FormatGray", FormatGray("x"), "x"},
			{"FormatGreen", FormatGreen("x"), "x"},
			{"FormatRed", FormatRed("x"), "x"},
			{"FormatYellow", FormatYellow("x"), "x"},
		} {
			if fn.got != fn.in {
				t.Errorf("%s with NO_COLOR path = %q, want %q", fn.name, fn.got, fn.in)
			}
		}
	})

	t.Run("with color", func(t *testing.T) {
		noColor = false
		t.Cleanup(func() { noColor = old })
		cases := []struct {
			fn   func(string) string
			code string
		}{
			{FormatCyan, ansiCyan},
			{FormatCyanBold, ansiBoldCyan},
			{FormatGray, ansiGray},
			{FormatGreen, ansiGreen},
			{FormatRed, ansiRed},
			{FormatYellow, ansiYellow},
		}
		for _, tc := range cases {
			got := tc.fn("msg")
			if !strings.HasPrefix(got, tc.code) || !strings.HasSuffix(got, ansiReset) {
				t.Errorf("colored format = %q, expected prefix %q and reset suffix", got, tc.code)
			}
		}
	})
}

func TestSuccessErrorWarnInfoBoldGray(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = true

	t.Run("stdout messages", func(t *testing.T) {
		Quiet = false
		out := captureStdout(t, func() {
			Success("done")
			Info("note")
			Bold("title")
			Gray("muted")
		})
		for _, want := range []string{"✔ done", "ℹ note", "title", "muted"} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout output missing %q, got %q", want, out)
			}
		}
	})

	t.Run("quiet suppresses stdout", func(t *testing.T) {
		Quiet = true
		out := captureStdout(t, func() {
			Success("hidden")
			Info("hidden")
			Bold("hidden")
			Gray("hidden")
		})
		if out != "" {
			t.Errorf("Quiet should suppress stdout, got %q", out)
		}
	})

	t.Run("stderr messages", func(t *testing.T) {
		Quiet = false
		errOut := captureStderr(t, func() {
			Error("fail")
			Warn("careful")
		})
		for _, want := range []string{"✖ fail", "⚠ careful"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("stderr output missing %q, got %q", want, errOut)
			}
		}
	})

	t.Run("colored output", func(t *testing.T) {
		noColor = false
		Quiet = false
		out := captureStdout(t, func() {
			Success("ok")
		})
		if !strings.Contains(out, "\033[32m") {
			t.Errorf("Success with color should contain green ANSI, got %q", out)
		}
	})
}

func TestStatusPriorityBadge_WithColor(t *testing.T) {
	old := noColor
	noColor = false
	t.Cleanup(func() { noColor = old })

	if !strings.Contains(StatusBadge("Done"), "\033[") {
		t.Error("StatusBadge should include ANSI codes when color enabled")
	}
	if !strings.Contains(PriorityBadge("High"), "\033[") {
		t.Error("PriorityBadge should include ANSI codes when color enabled")
	}
}
