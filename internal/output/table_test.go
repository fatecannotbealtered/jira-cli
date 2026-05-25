package output

import (
	"strings"
	"testing"
)

func TestTermWidth(t *testing.T) {
	t.Run("default fallback", func(t *testing.T) {
		old := termWidthFn
		termWidthFn = func() int { return 120 }
		t.Cleanup(func() { termWidthFn = old })

		if w := termWidth(); w != 120 {
			t.Errorf("termWidth() = %d, want 120", w)
		}
	})

	t.Run("detected width", func(t *testing.T) {
		old := termWidthFn
		termWidthFn = func() int { return 80 }
		t.Cleanup(func() { termWidthFn = old })

		if w := termWidth(); w != 80 {
			t.Errorf("termWidth() = %d, want 80", w)
		}
	})

	t.Run("real stdout", func(t *testing.T) {
		w := defaultTermWidth()
		if w <= 0 {
			t.Errorf("defaultTermWidth() = %d, want positive", w)
		}
	})

	t.Run("detected from GetSize", func(t *testing.T) {
		old := getTerminalSize
		getTerminalSize = func(int) (int, int, error) { return 100, 0, nil }
		t.Cleanup(func() { getTerminalSize = old })

		if w := defaultTermWidth(); w != 100 {
			t.Errorf("defaultTermWidth() = %d, want 100", w)
		}
	})
}

func TestTable_EarlyReturn(t *testing.T) {
	oldQuiet := Quiet
	t.Cleanup(func() { Quiet = oldQuiet })

	Quiet = true
	out := captureStdout(t, func() {
		Table([]string{"A", "B"}, [][]string{{"1", "2"}})
	})
	if out != "" {
		t.Errorf("Quiet Table should produce no output, got %q", out)
	}

	Quiet = false
	out = captureStdout(t, func() {
		Table(nil, nil)
	})
	if out != "" {
		t.Errorf("empty headers should produce no output, got %q", out)
	}
}

func TestTable_Rendering(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = true
	Quiet = false

	headers := []string{"Key", "Summary", "Status"}
	rows := [][]string{
		{"PROJ-1", "Fix bug", "Open"},
		{"PROJ-2", "Add feature", "Done"},
	}

	out := captureStdout(t, func() {
		Table(headers, rows)
	})

	for _, want := range []string{"┌", "├", "└", "│", "KEY", "PROJ-1", "PROJ-2", "Fix bug"} {
		if !strings.Contains(out, want) {
			t.Errorf("Table output missing %q\n%s", want, out)
		}
	}
}

func TestTable_CJKColumns(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = true
	Quiet = false

	out := captureStdout(t, func() {
		Table(
			[]string{"编号", "摘要"},
			[][]string{
				{"PROJ-1", "中文标题测试"},
				{"PROJ-2", "日本語テスト"},
			},
		)
	})

	if !strings.Contains(out, "编号") || !strings.Contains(out, "中文标题") {
		t.Errorf("CJK table should render content, got:\n%s", out)
	}
}

func TestTable_TruncationAndShrink(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = true
	Quiet = false

	longCell := strings.Repeat("W", 200)
	out := captureStdout(t, func() {
		Table(
			[]string{"ID", "Description"},
			[][]string{{"1", longCell}},
		)
	})

	if strings.Contains(out, longCell) {
		t.Error("long cell should be truncated in table output")
	}
	if !strings.Contains(out, "…") {
		t.Error("truncated cell should contain ellipsis")
	}
}

func TestTable_ShortRowCells(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = true
	Quiet = false

	out := captureStdout(t, func() {
		Table(
			[]string{"A", "B", "C"},
			[][]string{{"only-one"}},
		)
	})

	if !strings.Contains(out, "ONLY-ONE") && !strings.Contains(out, "only-one") {
		t.Errorf("row with fewer cells should still render, got:\n%s", out)
	}
}

func TestTable_ColoredHeaders(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
	})
	noColor = false
	Quiet = false

	out := captureStdout(t, func() {
		Table([]string{"Col"}, [][]string{{"val"}})
	})

	if !strings.Contains(out, "\033[") {
		t.Error("colored headers should contain ANSI escape sequences")
	}
}

func TestTable_ShrinkStopAtMinWidth(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	oldTermWidth := termWidthFn
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
		termWidthFn = oldTermWidth
	})
	noColor = true
	Quiet = false
	termWidthFn = func() int { return 30 }

	headers := make([]string, 8)
	rows := make([][]string, 1)
	rows[0] = make([]string, 8)
	for i := range headers {
		headers[i] = "Col"
		rows[0][i] = strings.Repeat("X", 20)
	}

	out := captureStdout(t, func() {
		Table(headers, rows)
	})
	if out == "" {
		t.Fatal("expected table output")
	}
}

func TestClampPadding(t *testing.T) {
	if got := clampPadding(5, 3); got != 2 {
		t.Errorf("clampPadding(5,3) = %d, want 2", got)
	}
	if got := clampPadding(3, 5); got != 0 {
		t.Errorf("clampPadding(3,5) = %d, want 0", got)
	}
}

func TestTable_HeaderPaddingClamp(t *testing.T) {
	oldNoColor := noColor
	oldQuiet := Quiet
	oldTermWidth := termWidthFn
	t.Cleanup(func() {
		noColor = oldNoColor
		Quiet = oldQuiet
		termWidthFn = oldTermWidth
	})
	noColor = true
	Quiet = false
	termWidthFn = func() int { return 8 }

	// "ßxx" uppercases to "SSxx" (wider than shrunk column width), exercising padding clamp.
	out := captureStdout(t, func() {
		Table(
			[]string{"ßxx", strings.Repeat("W", 10)},
			[][]string{{"data", "row"}},
		)
	})
	if out == "" {
		t.Fatal("expected table output")
	}
}
