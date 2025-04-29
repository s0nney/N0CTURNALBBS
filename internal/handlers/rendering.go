package handlers

import (
	"strings"
)

func FormatPreview(content string, maxLength int) string {
	if len(content) > maxLength {
		content = content[:maxLength] + "..."
	}

	content = (content)

	content = strings.ReplaceAll(content, "\n", " ")

	return content
}
