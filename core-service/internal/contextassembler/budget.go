// Turn-boundary cutting and budget arithmetic.
//
// Adapted from picoclaw pkg/agent/context_budget.go @ bbf6893 (MIT,
// Copyright (c) 2026 PicoClaw contributors) — rewritten against this
// repository's Message shape. The algorithm is kept: cuts land on user-role
// boundaries so a tool-call sequence (call → results) is never torn apart,
// and the budget is the three-term form messages + tools + reserve > window.
//
// What was deliberately NOT adopted from upstream: its heuristic tokenizer
// (2.5 chars/token). This module refuses to count without an exact tokenizer
// (D-C15); see ErrTokenizerUnavailable.

package contextassembler

import "strings"

// parseTurnBoundaries returns the starting index of each turn in msgs.
// A turn begins at a user message and extends through all subsequent
// assistant/tool messages until the next user message. Cutting at a turn
// boundary guarantees no tool-call sequence is split across the cut.
func parseTurnBoundaries(msgs []Message) []int {
	var starts []int
	for i, m := range msgs {
		if strings.EqualFold(m.Role, "user") {
			starts = append(starts, i)
		}
	}
	return starts
}

// nextTrimStart returns where the next trimming step should cut: the second
// turn boundary, so the oldest complete turn drops first and tool-call
// sequences stay intact.
func nextTrimStart(msgs []Message) int {
	turns := parseTurnBoundaries(msgs)
	switch {
	case len(turns) >= 2:
		return turns[1]
	case len(turns) == 1:
		if turns[0] > 0 {
			return turns[0]
		}
		return len(msgs)
	default:
		return len(msgs)
	}
}

// overBudget reports whether counted tokens exceed the effective budget.
func overBudget(budget, counted int) bool { return counted > budget }

// trimToBudget progressively drops the oldest complete turns until the kept
// messages fit. It returns the kept messages and everything that left.
// The preserved slice is NOT part of this computation's inputs — callers pass
// only the compactable half (D-C14).
func trimToBudget(msgs []Message, count func([]Message) int, budget int) (kept, dropped []Message) {
	if !overBudget(budget, count(msgs)) {
		return msgs, nil
	}
	kept = msgs
	for len(kept) > 0 {
		cut := nextTrimStart(kept)
		if cut <= 0 || cut >= len(kept) {
			return nil, append(dropped, kept...)
		}
		dropped = append(dropped, kept[:cut]...)
		kept = append([]Message(nil), kept[cut:]...)
		if !overBudget(budget, count(kept)) {
			return kept, dropped
		}
	}
	return nil, dropped
}
