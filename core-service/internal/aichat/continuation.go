package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/repository"
)

type continuationSeed struct {
	session             *repository.AIChatSession
	sourceRun           *repository.AIChatRun
	parentRunID         string
	effectiveContractID string
	pageContext         *PageContext
	history             []Message
	instruction         string
	target              Target
	compressionStrategy string
}

func (r *Runtime[T]) Continue(ctx context.Context, command ContinueCommand) (*Started, error) {
	seed, err := r.resolveContinuation(ctx, command)
	if err != nil {
		return nil, err
	}
	input := Input{
		SessionID: seed.session.ID, ParentRunID: seed.parentRunID,
		Message: seed.instruction, ContractID: seed.effectiveContractID,
		History: seed.history, Language: command.Language, PageContext: seed.pageContext,
		UserID: command.UserID, LegalEntityID: command.LegalEntityID,
		Role: command.Role, Permissions: append([]string(nil), command.Permissions...), AuthHeader: command.AuthHeader,
	}
	prepared, err := r.prepare(ctx, input, seed.session, seed.sourceRun)
	if err != nil {
		return nil, fmt.Errorf("prepare AI agent continuation: %w", err)
	}
	continuation := &Continuation{
		Target: seed.target, ParentRunID: seed.parentRunID,
		ResolvedInstruction: seed.instruction, EffectiveContractID: seed.effectiveContractID,
		ResolvedSkillID: prepared.plan.SkillID, HistoryMessageCount: len(seed.history),
		CompressionStrategy: seed.compressionStrategy,
	}
	started := r.started(prepared, continuation)
	r.dispatch(func() {
		// Preserve the authenticated scope for the detached continuation just as
		// Start does. A follow-up must not widen access merely because it is
		// resumed from a run/artifact target.
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
		defer cancel()
		r.executeDispatched(runCtx, prepared)
	})
	return started, nil
}

func (r *Runtime[T]) resolveContinuation(ctx context.Context, command ContinueCommand) (*continuationSeed, error) {
	target := Target{Type: strings.ToLower(strings.TrimSpace(command.Target.Type)), ID: strings.TrimSpace(command.Target.ID)}
	if command.UserID == "" {
		return nil, fmt.Errorf("AI agent continuation requires a user")
	}
	if target.Type == "" || target.ID == "" {
		return nil, fmt.Errorf("AI agent continuation target is required")
	}
	if command.Language == "" {
		command.Language = "zh-CN"
	}

	var session *repository.AIChatSession
	var sourceRun *repository.AIChatRun
	var parentRunID, anchorMessageID, instruction string
	var pageContext *PageContext
	var fallbackContractIDs []string

	boundary, boundaryErr := entityBoundary(ctx)
	if boundaryErr != nil {
		return nil, fmt.Errorf("resolve AI chat continuation boundary: %w", boundaryErr)
	}
	switch target.Type {
	case "run":
		run, err := r.store.GetRunByID(ctx, target.ID, command.UserID)
		if err != nil {
			return nil, fmt.Errorf("load continuation run: %w", err)
		}
		session, err = r.store.GetSessionByID(ctx, run.SessionID, command.UserID, boundary)
		if err != nil {
			return nil, fmt.Errorf("load continuation session: %w", err)
		}
		sourceRun, parentRunID = run, run.ID
		pageContext = parsePageContext(run.PageContext)
		if run.TriggerMessageID != nil {
			anchorMessageID = *run.TriggerMessageID
		}
		instruction = continuationInstruction("run", summarizeRun(run))
	case "message":
		message, err := r.store.GetMessageByID(ctx, target.ID, command.UserID)
		if err != nil {
			return nil, fmt.Errorf("load continuation message: %w", err)
		}
		session, err = r.store.GetSessionByID(ctx, message.SessionID, command.UserID, boundary)
		if err != nil {
			return nil, fmt.Errorf("load continuation session: %w", err)
		}
		anchorMessageID = message.ID
		if message.RunID != nil && *message.RunID != "" {
			parentRunID = *message.RunID
			sourceRun, _ = r.store.GetRunByID(ctx, parentRunID, command.UserID)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
			}
		}
		instruction = continuationInstruction("message", fmt.Sprintf("role=%s, content=%s", message.Role, truncate(message.Content, 120)))
	case "artifact":
		artifact, err := r.store.GetArtifactByID(ctx, target.ID, command.UserID)
		if err != nil {
			return nil, fmt.Errorf("load continuation artifact: %w", err)
		}
		session, err = r.store.GetSessionByID(ctx, artifact.SessionID, command.UserID, boundary)
		if err != nil {
			return nil, fmt.Errorf("load continuation session: %w", err)
		}
		parentRunID = artifact.RunID
		if parentRunID != "" {
			sourceRun, _ = r.store.GetRunByID(ctx, parentRunID, command.UserID)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
				if sourceRun.TriggerMessageID != nil {
					anchorMessageID = *sourceRun.TriggerMessageID
				}
			}
		}
		fallbackContractIDs = append(fallbackContractIDs, contractIDFromJSON(artifact.Data))
		instruction = continuationInstruction("artifact", summarizeArtifact(artifact))
	case "action":
		action, err := r.store.GetReviewActionByID(ctx, target.ID, command.UserID)
		if err != nil {
			return nil, fmt.Errorf("load continuation review action: %w", err)
		}
		session, err = r.store.GetSessionByID(ctx, action.SessionID, command.UserID, boundary)
		if err != nil {
			return nil, fmt.Errorf("load continuation session: %w", err)
		}
		var artifact *repository.AIChatArtifact
		if action.ArtifactID != nil && *action.ArtifactID != "" {
			artifact, _ = r.store.GetArtifactByID(ctx, *action.ArtifactID, command.UserID)
			if artifact != nil {
				parentRunID = artifact.RunID
				fallbackContractIDs = append(fallbackContractIDs, contractIDFromJSON(artifact.Data))
			}
		}
		if parentRunID == "" && action.RunID != nil {
			parentRunID = *action.RunID
		}
		if parentRunID != "" {
			sourceRun, _ = r.store.GetRunByID(ctx, parentRunID, command.UserID)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
				if sourceRun.TriggerMessageID != nil {
					anchorMessageID = *sourceRun.TriggerMessageID
				}
			}
		}
		fallbackContractIDs = append(fallbackContractIDs, contractIDFromJSON(action.ActionPayload))
		instruction = continuationInstruction("action", summarizeAction(action, artifact))
	default:
		return nil, fmt.Errorf("unsupported AI agent continuation target: %s", command.Target.Type)
	}

	messages, err := r.store.ListMessagesBySession(ctx, session.ID, 100)
	if err != nil {
		return nil, fmt.Errorf("load AI chat continuation history: %w", err)
	}
	history, strategy := compressedHistory(messages, anchorMessageID)
	pageContext = mergePageContext(command.PageContext, pageContext)
	effectiveContractID := resolveContractID(command.ContractID, pageContext, session, fallbackContractIDs...)
	if strings.TrimSpace(command.Instruction) != "" {
		instruction = strings.TrimSpace(command.Instruction)
	}
	return &continuationSeed{
		session: session, sourceRun: sourceRun, parentRunID: parentRunID,
		effectiveContractID: effectiveContractID, pageContext: pageContext,
		history: history, instruction: instruction, target: target,
		compressionStrategy: strategy,
	}, nil
}

func compressedHistory(messages []*repository.AIChatMessage, anchorMessageID string) ([]Message, string) {
	ascending := make([]Message, 0, len(messages))
	messageIDs := make([]string, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		ascending = append(ascending, Message{Role: message.Role, Content: message.Content})
		messageIDs = append(messageIDs, message.ID)
	}
	if len(ascending) == 0 {
		return nil, "empty"
	}
	if len(ascending) <= 12 {
		return ascending, "full_session"
	}
	if anchorMessageID == "" {
		return ascending[len(ascending)-12:], "session_tail"
	}
	anchor := -1
	for index, id := range messageIDs {
		if id == anchorMessageID {
			anchor = index
			break
		}
	}
	if anchor == -1 {
		return ascending[len(ascending)-12:], "session_tail"
	}
	start := anchor - 8
	if start < 0 {
		start = 0
	}
	end := start + 12
	if end > len(ascending) {
		end = len(ascending)
		start = end - 12
	}
	return ascending[start:end], "anchor_window"
}

func parsePageContext(raw json.RawMessage) *PageContext {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value PageContext
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}

func mergePageContext(primary, fallback *PageContext) *PageContext {
	if primary != nil {
		return primary
	}
	return fallback
}

func resolveContractID(explicit string, pageContext *PageContext, session *repository.AIChatSession, fallbacks ...string) string {
	if explicit != "" {
		return explicit
	}
	if pageContext != nil && pageContext.ContractID != "" {
		return pageContext.ContractID
	}
	if session != nil && session.BoundContractID != nil && *session.BoundContractID != "" {
		return *session.BoundContractID
	}
	for _, candidate := range fallbacks {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func contractIDFromJSON(raw json.RawMessage) string {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	value, _ := payload["contract_id"].(string)
	return value
}

func continuationInstruction(targetType, summary string) string {
	base := "请基于当前会话上下文继续推进，先概括当前状态，再给出下一步建议；如果仍需人工确认，请明确指出。"
	if summary == "" {
		return base
	}
	return fmt.Sprintf("请承接刚才的%s继续推进。当前锚点信息：%s。%s", targetType, summary, base)
}

func summarizeRun(run *repository.AIChatRun) string {
	if run == nil {
		return ""
	}
	parts := []string{}
	if run.SkillID != nil && *run.SkillID != "" {
		parts = append(parts, fmt.Sprintf("skill=%s", *run.SkillID))
	}
	if run.Status != "" {
		parts = append(parts, fmt.Sprintf("status=%s", run.Status))
	}
	if run.SummaryText != nil && *run.SummaryText != "" {
		parts = append(parts, fmt.Sprintf("summary=%s", truncate(*run.SummaryText, 120)))
	}
	return strings.Join(parts, ", ")
}

func summarizeArtifact(artifact *repository.AIChatArtifact) string {
	if artifact == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("artifact_type=%s", artifact.ArtifactType)}
	if artifact.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%s", artifact.Title))
	}
	if artifact.Status != "" {
		parts = append(parts, fmt.Sprintf("status=%s", artifact.Status))
	}
	var data map[string]any
	if json.Unmarshal(artifact.Data, &data) == nil {
		if contracts, ok := data["contracts"].([]any); ok {
			parts = append(parts, fmt.Sprintf("contracts=%d", len(contracts)))
		}
		if schedules, ok := data["schedules"].([]any); ok {
			parts = append(parts, fmt.Sprintf("schedules=%d", len(schedules)))
		}
	}
	return strings.Join(parts, ", ")
}

func summarizeAction(action *repository.AIChatReviewAction, artifact *repository.AIChatArtifact) string {
	if action == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("action=%s", action.ActionType)}
	if artifact != nil {
		parts = append(parts, summarizeArtifact(artifact))
	}
	if action.Comment != nil && *action.Comment != "" {
		parts = append(parts, fmt.Sprintf("comment=%s", truncate(*action.Comment, 80)))
	}
	return strings.Join(parts, ", ")
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit]) + "..."
}
