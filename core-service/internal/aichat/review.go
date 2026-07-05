package aichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/repository"
)

func (r *Runtime[T]) Review(ctx context.Context, command ReviewCommand) (*ReviewResult, error) {
	if strings.TrimSpace(command.UserID) == "" {
		return nil, fmt.Errorf("AI agent review requires a user")
	}
	if strings.TrimSpace(command.ArtifactID) == "" {
		return nil, fmt.Errorf("AI agent review requires an artifact")
	}
	if strings.TrimSpace(command.ActionType) == "" {
		return nil, fmt.Errorf("AI agent review requires an action")
	}
	artifact, err := r.store.GetArtifactByID(ctx, command.ArtifactID, command.UserID)
	if err != nil {
		return nil, fmt.Errorf("load AI agent artifact: %w", err)
	}
	action := &repository.AIChatReviewAction{
		SessionID: artifact.SessionID, RunID: &artifact.RunID, ArtifactID: &artifact.ID,
		ActionType: command.ActionType, ActionPayload: marshalJSON(command.ActionPayload),
		ActedBy: command.UserID,
	}
	if command.Comment != "" {
		action.Comment = &command.Comment
	}
	if err := r.store.RecordReviewAction(ctx, action); err != nil {
		return nil, fmt.Errorf("record AI agent review action: %w", err)
	}
	status := artifactStatus(command.ActionType)
	if err := r.store.UpdateArtifactStatus(ctx, artifact.ID, status); err != nil {
		return nil, fmt.Errorf("update AI agent artifact status: %w", err)
	}
	result := &ReviewResult{Action: action, ArtifactID: artifact.ID, ArtifactStatus: status}
	if command.FollowUp != nil {
		followUp := *command.FollowUp
		followUp.Target = Target{Type: "action", ID: action.ID}
		if followUp.UserID == "" {
			followUp.UserID = command.UserID
		}
		started, err := r.Continue(ctx, followUp)
		if err != nil {
			return nil, fmt.Errorf("start AI agent review continuation: %w", err)
		}
		result.FollowUp = started
	}
	return result, nil
}

func artifactStatus(actionType string) string {
	switch actionType {
	case "confirm", "import", "create_draft":
		return "confirmed"
	case "reject":
		return "rejected"
	case "skip":
		return "archived"
	default:
		return "draft"
	}
}
