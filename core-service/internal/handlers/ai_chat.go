package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

// aiChatRuntimeStore is the read-side seam used by the HTTP adapters. Run
// transitions and writes live behind aichat.Runtime's interface.
type aiChatRuntimeStore interface {
	GetSessionByID(context.Context, string, string) (*repository.AIChatSession, error)
	ListSessions(context.Context, repository.AIChatSessionFilter) ([]*repository.AIChatSession, error)
	GetRunByID(context.Context, string, string) (*repository.AIChatRun, error)
	ListRunsBySession(context.Context, string, int, int) ([]*repository.AIChatRun, error)
	ListMessagesBySession(context.Context, string, int) ([]*repository.AIChatMessage, error)
	ListRunEvents(context.Context, string, int, int) ([]*repository.AIChatRunEvent, error)
	ListArtifactsBySession(context.Context, string, int) ([]*repository.AIChatArtifact, error)
	ListReviewActionsBySession(context.Context, string, int) ([]*repository.AIChatReviewAction, error)
}

type AIChatHandler struct {
	runtimeRepo  aiChatRuntimeStore
	agentRuntime *aichat.Runtime[aiagent.Response]
}

func NewAIChatHandler(
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	runtimeRepo *repository.AIChatRuntimeRepository,
) *AIChatHandler {
	agent := aiagent.New(contractRepo, mcRepo, eventRepo)
	return &AIChatHandler{
		runtimeRepo: runtimeRepo,
		agentRuntime: aichat.NewRuntime(
			runtimeRepo,
			agent,
			agent,
			aiagent.ProjectResult,
			aichat.Options{},
		),
	}
}

type PageContext = aichat.PageContext
type ChatMessage = aichat.Message
type AIChatRequest = aiagent.Request

func (h *AIChatHandler) Chat(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}
	completed, err := h.agentRuntime.Run(c.Request.Context(), runtimeInput(c, req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to run AI agent: " + err.Error()})
		return
	}
	response := completed.Response
	response.SessionID = completed.Started.Run.SessionID
	response.RunID = completed.Started.Run.ID
	c.JSON(http.StatusOK, response)
}

func runtimeInput(c *gin.Context, req AIChatRequest) aichat.Input {
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	role, _ := c.Get("role")
	roleString, _ := role.(string)
	return aichat.Input{
		SessionID: req.SessionID, Message: req.Message,
		ContractID: req.ContractID, History: req.History,
		FileID: req.FileID, ObjectName: req.ObjectName, ContentType: req.ContentType,
		Language: req.Language, PageContext: req.PageContext,
		UserID: userIDString, LegalEntityID: middleware.GetTenantID(c),
		Role: roleString, AuthHeader: c.GetHeader("Authorization"),
	}
}
