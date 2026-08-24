// First-party replacement for picoclaw pkg/utils: the helpers the vendored
// agent kernel slice calls (bodies mirror upstream utils/string.go etc., commit
// bbf6893ca7afad27f1d00a0f5a45982a549c6ed6). NOT new code.
package utils

import "strings"

// Truncate shortens s to maxLen runes, appending an ellipsis.
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
