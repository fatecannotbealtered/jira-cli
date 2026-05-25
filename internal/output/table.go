package output

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/term"
)

// ansiEscapeRe matches ANSI escape sequences.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// termWidthFn returns terminal width; replaced in tests.
var termWidthFn = defaultTermWidth

// getTerminalSize wraps term.GetSize for test overrides.
var getTerminalSize = term.GetSize

func defaultTermWidth() int {
	if w, _, err := getTerminalSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 120
}

// termWidth returns the terminal width, defaulting to 120 if unavailable.
func termWidth() int {
	return termWidthFn()
}

// runeWidth calculates the display width of a string (CJK characters count as 2 columns, others as 1).
func runeWidth(s string) int {
	width := 0
	for _, r := range s {
		if isCJK(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// isCJK returns true if the rune is a CJK (Chinese/Japanese/Korean) wide character.
func isCJK(r rune) bool {
	if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
		return true
	}
	// CJK symbols, punctuation, compatibility, and extension blocks
	return (r >= 0x2E80 && r <= 0x2EFF) || // CJK Radicals Supplement
		(r >= 0x2F00 && r <= 0x2FDF) || // Kangxi Radicals
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols and Punctuation
		(r >= 0x3200 && r <= 0x32FF) || // Enclosed CJK Letters
		(r >= 0x3300 && r <= 0x33FF) || // CJK Compatibility
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE30 && r <= 0xFE4F) || // CJK Compatibility Forms
		(r >= 0xFF01 && r <= 0xFF60) || // Fullwidth ASCII
		(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Signs
		(r >= 0x20000 && r <= 0x2A6DF) // CJK Unified Ideographs Extension B
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// truncate truncates a string to the given display width, appending "…" if it overflows.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	plain := stripAnsi(s)
	if runeWidth(plain) <= maxWidth {
		return s
	}

	// Truncate character by character
	width := 0
	var buf strings.Builder
	for _, r := range plain {
		rw := 1
		if isCJK(r) {
			rw = 2
		}
		if width+rw > maxWidth-1 { // reserve 1 column for "…"
			break
		}
		buf.WriteRune(r)
		width += rw
	}
	buf.WriteRune('\u2026')
	return buf.String()
}

func clampPadding(colWidth, displayWidth int) int {
	padding := colWidth - displayWidth
	if padding < 0 {
		return 0
	}
	return padding
}

// Table prints a bordered, colored table.
// headers: column titles (auto-uppercased and styled with cyan+bold)
// rows: data rows
func Table(headers []string, rows [][]string) {
	if Quiet || len(headers) == 0 {
		return
	}

	cols := len(headers)

	// Calculate the maximum content width per column (display width after stripping ANSI codes)
	colWidths := make([]int, cols)
	for i, h := range headers {
		w := runeWidth(strings.ToUpper(h))
		if w > colWidths[i] {
			colWidths[i] = w
		}
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			w := runeWidth(stripAnsi(row[i]))
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Calculate total width: borders + column widths + 1 space padding on each side
	// Format: │ col │ col │ ... │
	// Total = 1 + sum(colWidths[i] + 2) + (cols-1)*1 + 1
	//       = 2 + cols + sum(colWidths)
	totalWidth := 2 + cols
	for _, w := range colWidths {
		totalWidth += w
	}

	// If exceeding terminal width, proportionally shrink the widest column
	tw := termWidth()
	for totalWidth > tw && tw > 0 {
		// Find the widest column
		maxIdx := 0
		for i := 1; i < cols; i++ {
			if colWidths[i] > colWidths[maxIdx] {
				maxIdx = i
			}
		}
		if colWidths[maxIdx] <= 3 {
			break // stop shrinking
		}
		colWidths[maxIdx]--
		totalWidth--
	}

	// Build horizontal separator lines
	buildHLine := func(left, mid, right, fill string) string {
		var sb strings.Builder
		sb.WriteString(left)
		for i, w := range colWidths {
			sb.WriteString(strings.Repeat(fill, w+2))
			if i < cols-1 {
				sb.WriteString(mid)
			}
		}
		sb.WriteString(right)
		return sb.String()
	}

	topLine := buildHLine("┌", "┬", "┐", "─")
	midLine := buildHLine("├", "┼", "┤", "─")
	botLine := buildHLine("└", "┴", "┘", "─")

	// Build a single row
	buildRow := func(cells []string, isHeader bool) string {
		var sb strings.Builder
		sb.WriteString("│")
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}

			// Truncate
			plain := stripAnsi(cell)
			if runeWidth(plain) > colWidths[i] {
				cell = truncate(plain, colWidths[i])
				plain = stripAnsi(cell)
			}

			// Colorize header
			if isHeader {
				cell = FormatCyanBold(strings.ToUpper(plain))
			}

			// Right-pad with spaces (based on display width)
			displayWidth := runeWidth(stripAnsi(cell))
			padding := clampPadding(colWidths[i], displayWidth)

			sb.WriteString(" ")
			sb.WriteString(cell)
			sb.WriteString(strings.Repeat(" ", padding))
			sb.WriteString(" │")
		}
		return sb.String()
	}

	fmt.Println(topLine)
	fmt.Println(buildRow(headers, true))
	fmt.Println(midLine)
	for _, row := range rows {
		fmt.Println(buildRow(row, false))
	}
	fmt.Println(botLine)
}
