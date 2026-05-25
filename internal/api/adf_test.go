package api

import (
	"testing"
)

func buildDoc(content ...any) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

func paragraphNode(content ...any) map[string]any {
	return map[string]any{
		"type":    "paragraph",
		"content": content,
	}
}

func textNode(t string) map[string]any {
	return map[string]any{"type": "text", "text": t}
}

func hardBreakNode() map[string]any {
	return map[string]any{"type": "hardBreak"}
}

func headingNode(level int, content ...any) map[string]any {
	return map[string]any{
		"type":    "heading",
		"attrs":   map[string]any{"level": level},
		"content": content,
	}
}

func bulletListNode(items ...any) map[string]any {
	return map[string]any{"type": "bulletList", "content": items}
}

func orderedListNode(items ...any) map[string]any {
	return map[string]any{"type": "orderedList", "content": items}
}

func listItemNode(content ...any) map[string]any {
	return map[string]any{"type": "listItem", "content": content}
}

func codeBlockNode(code string) map[string]any {
	return map[string]any{
		"type":    "codeBlock",
		"content": []any{textNode(code)},
	}
}

func mentionNode(displayName string) map[string]any {
	return map[string]any{
		"type":  "mention",
		"attrs": map[string]any{"text": displayName},
	}
}

func TestADFToText_NilInput(t *testing.T) {
	if got := ADFToText(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestADFToText_NonMapInput(t *testing.T) {
	if got := ADFToText("not a map"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestADFToText_SimpleParagraph(t *testing.T) {
	doc := buildDoc(paragraphNode(textNode("Hello, World!")))
	if got := ADFToText(doc); got != "Hello, World!" {
		t.Errorf("got %q", got)
	}
}

func TestADFToText_MultipleParagraphs(t *testing.T) {
	doc := buildDoc(
		paragraphNode(textNode("First")),
		paragraphNode(textNode("Second")),
	)
	want := "First\nSecond"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_HardBreak(t *testing.T) {
	doc := buildDoc(paragraphNode(textNode("line1"), hardBreakNode(), textNode("line2")))
	want := "line1\nline2"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_BulletList(t *testing.T) {
	doc := buildDoc(bulletListNode(
		listItemNode(paragraphNode(textNode("item one"))),
		listItemNode(paragraphNode(textNode("item two"))),
	))
	want := "\u2022 item one\n\u2022 item two"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_OrderedList(t *testing.T) {
	doc := buildDoc(orderedListNode(
		listItemNode(paragraphNode(textNode("first"))),
		listItemNode(paragraphNode(textNode("second"))),
	))
	want := "1. first\n2. second"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_Heading(t *testing.T) {
	doc := buildDoc(headingNode(2, textNode("My Heading")))
	want := "## My Heading"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_CodeBlock(t *testing.T) {
	doc := buildDoc(codeBlockNode("fmt.Println(\"hello\")"))
	want := "```\nfmt.Println(\"hello\")\n```"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_Mention(t *testing.T) {
	doc := buildDoc(paragraphNode(mentionNode("John Doe")))
	want := "@John Doe"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_Emoji(t *testing.T) {
	doc := buildDoc(paragraphNode(map[string]any{
		"type":  "emoji",
		"attrs": map[string]any{"shortName": ":smile:"},
	}))
	want := ":smile:"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTextToADF_Structure(t *testing.T) {
	adf := TextToADF("hello")
	if adf["type"] != "doc" {
		t.Errorf("expected type=doc, got %v", adf["type"])
	}
	if adf["version"] != 1 {
		t.Errorf("expected version=1, got %v", adf["version"])
	}
	content := adf["content"].([]any)
	para := content[0].(map[string]any)
	if para["type"] != "paragraph" {
		t.Errorf("expected paragraph, got %v", para["type"])
	}
	paraContent := para["content"].([]any)
	textN := paraContent[0].(map[string]any)
	if textN["text"] != "hello" {
		t.Errorf("expected text=hello, got %v", textN["text"])
	}
}

func TestADFToText_RoundTrip(t *testing.T) {
	cases := []string{"simple text", "hello world", "", "multi word sentence"}
	for _, tc := range cases {
		adf := TextToADF(tc)
		if got := ADFToText(adf); got != tc {
			t.Errorf("roundtrip(%q) = %q", tc, got)
		}
	}
}

func TestADFToText_Blockquote(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type": "blockquote",
		"content": []any{
			paragraphNode(textNode("quoted line")),
		},
	})
	want := "> quoted line"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_Rule(t *testing.T) {
	doc := buildDoc(map[string]any{"type": "rule"})
	want := "---"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_MentionDisplayNameFallback(t *testing.T) {
	doc := buildDoc(paragraphNode(map[string]any{
		"type":  "mention",
		"attrs": map[string]any{"displayName": "Jane Smith"},
	}))
	want := "@Jane Smith"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_EmojiWithoutColons(t *testing.T) {
	doc := buildDoc(paragraphNode(map[string]any{
		"type":  "emoji",
		"attrs": map[string]any{"shortName": "thumbsup"},
	}))
	want := ":thumbsup:"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_EmojiEmptyShortName(t *testing.T) {
	doc := buildDoc(paragraphNode(map[string]any{
		"type":  "emoji",
		"attrs": map[string]any{"shortName": ""},
	}))
	if got := ADFToText(doc); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestADFToText_UnknownNodeType(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type":    "customPanel",
		"content": []any{paragraphNode(textNode("nested"))},
	})
	want := "nested"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_RenderChildrenSkipsNonMap(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type":    "paragraph",
		"content": []any{"not-a-map", textNode("kept")},
	})
	want := "kept"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_ListItemDirect(t *testing.T) {
	doc := buildDoc(listItemNode(paragraphNode(textNode("direct item"))))
	want := "direct item"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func tableRowNode(cells ...any) map[string]any {
	return map[string]any{"type": "tableRow", "content": cells}
}

func tableCellNode(content ...any) map[string]any {
	return map[string]any{"type": "tableCell", "content": content}
}

func TestADFToText_Table(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type": "table",
		"content": []any{
			tableRowNode(
				tableCellNode(paragraphNode(textNode("A1"))),
				tableCellNode(paragraphNode(textNode("B1"))),
			),
			tableRowNode(
				tableCellNode(paragraphNode(textNode("A2"))),
				tableCellNode(paragraphNode(textNode("B2"))),
			),
		},
	})
	want := "A1\tB1\nA2\tB2"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_TableSkipsInvalidNodes(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type": "table",
		"content": []any{
			"bad-row",
			map[string]any{"type": "tableHeader", "content": []any{}},
			tableRowNode(
				"bad-cell",
				tableCellNode(paragraphNode(textNode("only"))),
			),
		},
	})
	want := "only"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_ListSkipsNonMapItem(t *testing.T) {
	doc := buildDoc(bulletListNode(
		"bad-item",
		listItemNode(paragraphNode(textNode("ok"))),
	))
	want := "\u2022 ok"
	if got := ADFToText(doc); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestADFToText_InvalidContentType(t *testing.T) {
	doc := buildDoc(map[string]any{
		"type":    "paragraph",
		"content": "not-an-array",
	})
	if got := ADFToText(doc); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
