package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
)

// runEventRow is the minimal projection the CLI formatters need from an
// agent run event.
type runEventRow struct {
	Sequence   int             `json:"sequence_no"`
	EventType  string          `json:"event_type"`
	IsTerminal bool            `json:"is_terminal"`
	Payload    json.RawMessage `json:"payload"`
}

// formatEventsNDJSON emits one JSON event per line.
func formatEventsNDJSON(body []byte) (string, error) {
	events, err := decodeEventRows(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range events {
		raw, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// formatEventsTable emits a human-readable aligned table. Success data stays
// on stdout; the terminal markers render as ✓ / ✗.
func formatEventsTable(body []byte) (string, error) {
	events, err := decodeEventRows(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 4, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NO\tEVENT\tTERMINAL\tSUMMARY")
	for _, e := range events {
		marker := "✓"
		if e.IsTerminal {
			marker = "✗"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", e.Sequence, e.EventType, marker, summaryOf(e))
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	if len(events) == 0 {
		b.WriteString("(no events)\n")
	}
	return b.String(), nil
}

func summaryOf(e runEventRow) string {
	if len(e.Payload) == 0 || string(e.Payload) == "null" {
		return ""
	}
	var probe map[string]any
	if json.Unmarshal(e.Payload, &probe) != nil {
		return strings.ReplaceAll(truncateText(string(e.Payload)), "\n", " ")
	}
	if message, ok := probe["message"]; ok {
		if content, ok := message.(map[string]any); ok {
			if text, ok := content["content"].(string); ok {
				return truncateText(text)
			}
		}
	}
	if tool, ok := probe["tool"].(string); ok && tool != "" {
		status, _ := probe["status"].(string)
		return tool + " · " + status
	}
	return truncateText(string(e.Payload))
}

func decodeEventRows(body []byte) ([]runEventRow, error) {
	var envelope struct {
		Events []runEventRow `json:"events"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("bad events response: %w", err)
	}
	return envelope.Events, nil
}

func truncateText(s string) string {
	runes := []rune(s)
	if len(runes) <= 80 {
		return s
	}
	return string(runes[:80]) + "…"
}
