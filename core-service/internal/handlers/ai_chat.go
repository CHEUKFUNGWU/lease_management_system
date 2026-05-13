package handlers

import (
	"fmt"
	"net/http"
	"strings"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type AIChatHandler struct {
	contractRepo *repository.ContractRepository
	mcRepo       *repository.MonthlyClosingRepository
}

func NewAIChatHandler(contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository) *AIChatHandler {
	return &AIChatHandler{contractRepo: contractRepo, mcRepo: mcRepo}
}

type AIChatRequest struct {
	Message     string        `json:"message" binding:"required"`
	ContractID  string        `json:"contract_id,omitempty"`
	History     []ChatMessage `json:"history,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIChatResponse struct {
	Answer     string   `json:"answer"`
	Sources    []Source `json:"sources"`
	Confidence float64  `json:"confidence"`
	IsOfficial bool     `json:"is_official"`
}

type Source struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func (h *AIChatHandler) Chat(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	var sources []Source
	var answer strings.Builder
	
	legalEntityID := middleware.GetTenantID(c)
	if containsAny(req.Message, []string{"合同", "租赁", "门店", "租金"}) {
		contracts, _ := h.contractRepo.GetAll(ctx, legalEntityID)
		if len(contracts) > 0 {
			answer.WriteString(fmt.Sprintf("当前共有 %d 份合同。\n\n", len(contracts)))
			for _, contract := range contracts {
				sources = append(sources, Source{
					Type:    "contract",
					ID:      contract.ID,
					Title:   contract.ContractName,
					Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
				})
			}
		}
	}
	
	if containsAny(req.Message, []string{"负债", "折旧", "利息", "摊销", "ROU"}) {
		if req.ContractID != "" {
			results, _ := h.mcRepo.GetMeasurementResults(ctx, req.ContractID, "")
			if len(results) > 0 {
				latest := results[len(results)-1]
				answer.WriteString(fmt.Sprintf("**最新计量**（%s）：\n", latest.AccountingPeriod))
				answer.WriteString(fmt.Sprintf("- 期末负债: %.2f\n", latest.ClosingLiability))
				answer.WriteString(fmt.Sprintf("- 本期利息: %.2f\n", latest.InterestExpense))
				answer.WriteString(fmt.Sprintf("- 本期折旧: %.2f\n", latest.Depreciation))
				answer.WriteString(fmt.Sprintf("- 期末ROU: %.2f\n", latest.ClosingROUAsset))
			}
		}
	}
	
	if answer.Len() == 0 {
		answer.WriteString("我是 IFRS 16 智能助手。您可以问我：\n")
		answer.WriteString("- \"当前有多少份合同？\"\n")
		answer.WriteString("- \"合同 [ID] 的最新计量结果\"\n")
		answer.WriteString("- \"某期间的会计分录\"\n")
	}
	
	c.JSON(http.StatusOK, AIChatResponse{
		Answer:     answer.String(),
		Sources:    sources,
		Confidence: 0.85,
		IsOfficial: false,
	})
}

func containsAny(msg string, targets []string) bool {
	for _, t := range targets {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(t)) {
			return true
		}
	}
	return false
}
