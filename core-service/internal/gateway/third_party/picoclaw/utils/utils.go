// First-party replacement for picoclaw pkg/utils: the three helpers the
// vendored channels call. Bodies mirror upstream utils/string.go,
// utils/media.go and utils/tool_feedback.go (same commit). NOT new code.
package utils

import "strings"

// Truncate shortens s to maxLen characters (runes), appending an ellipsis.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// SanitizeFilename keeps filesystem-safe characters in uploaded names.
func SanitizeFilename(filename string) string {
	replaced := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, filename)
	replaced = strings.TrimSpace(replaced)
	if replaced == "" {
		return "file"
	}
	return replaced
}

const toolFeedbackEllipsis = "\n…"

// FitToolFeedbackMessage clips tool feedback content so status lines survive
// platform length limits.
func FitToolFeedbackMessage(content string, maxLen int) string {
	if len([]rune(content)) <= maxLen {
		return content
	}
	head := Truncate(strings.TrimSpace(content), maxLen-2)
	return head + toolFeedbackEllipsis
}
