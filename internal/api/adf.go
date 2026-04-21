package api

import (
	"fmt"
	"strings"
)

// ADFToText converts Jira ADF (Atlassian Document Format) JSON to plain text.
func ADFToText(adf any) string {
	if adf == nil {
		return ""
	}
	node, ok := adf.(map[string]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	renderNode(&sb, node, 0)
	return strings.TrimRight(sb.String(), "\n")
}

func renderNode(sb *strings.Builder, node map[string]any, depth int) {
	nodeType, _ := node["type"].(string)
	content := getADFContent(node)

	switch nodeType {
	case "doc":
		renderChildren(sb, content, depth)

	case "paragraph":
		renderChildren(sb, content, depth)
		sb.WriteString("\n")

	case "text":
		text, _ := node["text"].(string)
		sb.WriteString(text)

	case "hardBreak":
		sb.WriteString("\n")

	case "heading":
		sb.WriteString("## ")
		renderChildren(sb, content, depth)
		sb.WriteString("\n")

	case "bulletList":
		renderListItems(sb, content, depth, false)

	case "orderedList":
		renderListItems(sb, content, depth, true)

	case "listItem":
		renderChildren(sb, content, depth)

	case "codeBlock":
		sb.WriteString("```\n")
		renderChildren(sb, content, depth)
		sb.WriteString("\n```\n")

	case "blockquote":
		var inner strings.Builder
		renderChildren(&inner, content, depth)
		lines := strings.Split(strings.TrimRight(inner.String(), "\n"), "\n")
		for _, line := range lines {
			sb.WriteString("> ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}

	case "mention":
		attrs, _ := node["attrs"].(map[string]any)
		displayName, _ := attrs["text"].(string)
		if displayName == "" {
			displayName, _ = attrs["displayName"].(string)
		}
		sb.WriteString("@")
		sb.WriteString(displayName)

	case "emoji":
		attrs, _ := node["attrs"].(map[string]any)
		shortName, _ := attrs["shortName"].(string)
		if shortName != "" {
			if !strings.HasPrefix(shortName, ":") {
				shortName = ":" + shortName + ":"
			}
			sb.WriteString(shortName)
		}

	case "rule":
		sb.WriteString("---\n")

	case "table":
		renderTable(sb, content, depth)

	default:
		renderChildren(sb, content, depth)
	}
}

func renderChildren(sb *strings.Builder, content []any, depth int) {
	for _, child := range content {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		renderNode(sb, childNode, depth)
	}
}

func renderListItems(sb *strings.Builder, content []any, depth int, ordered bool) {
	for i, child := range content {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if ordered {
			fmt.Fprintf(sb, "%d. ", i+1)
		} else {
			sb.WriteString("• ")
		}
		var inner strings.Builder
		renderChildren(&inner, getADFContent(childNode), depth+1)
		text := strings.TrimRight(inner.String(), "\n")
		sb.WriteString(text)
		sb.WriteString("\n")
	}
}

func renderTable(sb *strings.Builder, rows []any, depth int) {
	for _, row := range rows {
		rowNode, ok := row.(map[string]any)
		if !ok {
			continue
		}
		rowType, _ := rowNode["type"].(string)
		if rowType != "tableRow" {
			continue
		}
		cells := getADFContent(rowNode)
		var cellTexts []string
		for _, cell := range cells {
			cellNode, ok := cell.(map[string]any)
			if !ok {
				continue
			}
			var cellSB strings.Builder
			renderChildren(&cellSB, getADFContent(cellNode), depth+1)
			cellTexts = append(cellTexts, strings.TrimRight(cellSB.String(), "\n"))
		}
		sb.WriteString(strings.Join(cellTexts, "\t"))
		sb.WriteString("\n")
	}
}

func getADFContent(node map[string]any) []any {
	raw, ok := node["content"]
	if !ok {
		return nil
	}
	content, ok := raw.([]any)
	if !ok {
		return nil
	}
	return content
}

// TextToADF converts plain text to a minimal ADF structure.
func TextToADF(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}
