// First-party replacement for picoclaw pkg/logger: the eight call-sites the
// vendored channels use, backed by log/slog. NOT upstream code. Component is
// preserved as the slog component attribute so log triage keeps its shape.
package logger

import "log/slog"

func attrs(fields map[string]any) []any {
	out := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		out = append(out, key, value)
	}
	return out
}

func DebugC(component string, message string) {
	slog.Debug(message, "component", component)
}

func DebugCF(component string, message string, fields map[string]any) {
	slog.Debug(message, append([]any{"component", component}, attrs(fields)...)...)
}

func InfoC(component string, message string) {
	slog.Info(message, "component", component)
}

func InfoCF(component string, message string, fields map[string]any) {
	slog.Info(message, append([]any{"component", component}, attrs(fields)...)...)
}

func WarnC(component string, message string) {
	slog.Warn(message, "component", component)
}

func WarnCF(component string, message string, fields map[string]any) {
	slog.Warn(message, append([]any{"component", component}, attrs(fields)...)...)
}

func ErrorC(component string, message string) {
	slog.Error(message, "component", component)
}

func ErrorCF(component string, message string, fields map[string]any) {
	slog.Error(message, append([]any{"component", component}, attrs(fields)...)...)
}

func FatalCF(component string, message string, fields map[string]any) {
	slog.Error(message, append([]any{"component", component, "fatal", true}, attrs(fields)...)...)
}
