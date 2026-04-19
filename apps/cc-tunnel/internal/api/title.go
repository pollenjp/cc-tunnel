package api

import (
	"regexp"
	"strings"
)

var (
	reBold     = regexp.MustCompile(`\*{1,2}([^*\n]+)\*{1,2}`)
	reUndScore = regexp.MustCompile(`_{1,2}([^_\n]+)_{1,2}`)
	reCode     = regexp.MustCompile("`([^`\n]+)`")
	reSpaces   = regexp.MustCompile(`\s+`)
)

// generateTitle generates a conversation title from the latest assistant response text.
// It strips Markdown formatting, replaces newlines with spaces, and truncates to 60 runes.
func generateTitle(text string) string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Remove heading markers from each line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		stripped := strings.TrimLeft(line, "#")
		if len(stripped) < len(line) {
			stripped = strings.TrimLeft(stripped, " ")
		}
		lines[i] = stripped
	}
	text = strings.Join(lines, "\n")

	// Remove bold (**text** or *text*) and italic (_text_ or __text__)
	text = reBold.ReplaceAllString(text, "$1")
	text = reUndScore.ReplaceAllString(text, "$1")

	// Remove inline code (`text`)
	text = reCode.ReplaceAllString(text, "$1")

	// Replace newlines with spaces, normalize whitespace
	text = strings.ReplaceAll(text, "\n", " ")
	text = reSpaces.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	if text == "" {
		return "New Conversation"
	}

	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return text
}
