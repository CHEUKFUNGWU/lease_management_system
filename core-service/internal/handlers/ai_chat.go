package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type AIChatHandler struct {
	contractRepo *repository.ContractRepository
	mcRepo       *repository.MonthlyClosingRepository
	eventRepo    *repository.EventRepository
}

func NewAIChatHandler(contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository, eventRepo *repository.EventRepository) *AIChatHandler {
	return &AIChatHandler{contractRepo: contractRepo, mcRepo: mcRepo, eventRepo: eventRepo}
}

type PageContext struct {
	Page       string            `json:"page,omitempty"`
	Title      string            `json:"title,omitempty"`
	ContractID string            `json:"contract_id,omitempty"`
	Period     string            `json:"period,omitempty"`
	ReportView string            `json:"report_view,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
	Summary    string            `json:"summary,omitempty"`
}

type AIChatRequest struct {
	Message     string        `json:"message" binding:"required"`
	ContractID  string        `json:"contract_id,omitempty"`
	History     []ChatMessage `json:"history,omitempty"`
	FileID      string        `json:"file_id,omitempty"`
	ObjectName  string        `json:"object_name,omitempty"`
	ContentType string        `json:"content_type,omitempty"`
	PageContext *PageContext  `json:"page_context,omitempty"`
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
	Model      string   `json:"model,omitempty"`
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
	legalEntityID := middleware.GetTenantID(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)

	var sources []Source
	var contextData strings.Builder

	// Resolve effective contract ID: explicit req.ContractID takes priority over page context
	effectiveContractID := req.ContractID
	if effectiveContractID == "" && req.PageContext != nil && req.PageContext.ContractID != "" {
		effectiveContractID = req.PageContext.ContractID
	}

	// 1. Handle file upload if present — keep existing parse behavior
	if req.FileID != "" && req.ObjectName != "" {
		parsed, err := h.parseFile(c, req.FileID, req.ObjectName, req.ContentType)
		if err != nil {
			contextData.WriteString(fmt.Sprintf("\n## 文件解析失败\n错误: %s\n", err.Error()))
		} else {
			contextData.WriteString("\n## 上传文件解析结果\n")
			for k, v := range parsed {
				contextData.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
			}
			sources = append(sources, Source{
				Type:    "file",
				ID:      req.FileID,
				Title:   "上传文件",
				Snippet: req.ObjectName,
			})
		}
	}

	// 2. Page-context-aware data retrieval
	// 2a. Contract detail context: always retrieve full contract data regardless of message keywords
	if effectiveContractID != "" {
		h.retrieveContractDetail(ctx, effectiveContractID, legalEntityID, &contextData, &sources)
	}

	// 2b. Reports page context
	if req.PageContext != nil && req.PageContext.Page == "reports" {
		h.appendReportsContext(req.PageContext, &contextData)
	}

	// 2c. Monthly closing page context
	if req.PageContext != nil && req.PageContext.Page == "monthly-closing" {
		h.appendMonthlyClosingContext(ctx, req.PageContext, &contextData, &sources)
	}

	// 3. Keyword-based retrieval (backward compatible, works alongside page context)
	msgLower := strings.ToLower(req.Message)

	// Contract list queries (only if no specific contract ID is resolved)
	if effectiveContractID == "" && containsAny(msgLower, []string{"合同", "租赁", "门店", "承租", "出租", "lease", "contract"}) {
		contracts, err := h.contractRepo.GetAll(ctx, legalEntityID)
		if err == nil && len(contracts) > 0 {
			contextData.WriteString(fmt.Sprintf("\n## 合同数据（共 %d 份）\n", len(contracts)))
			for _, contract := range contracts {
				contextData.WriteString(fmt.Sprintf("- ID: %s, 编号: %s, 名称: %s, 状态: %s, 承租方: %s, 出租方: %s\n",
					contract.ID, contract.ContractNumber, contract.ContractName,
					contract.ApprovalStatus, contract.LesseeName, contract.LessorName))
				sources = append(sources, Source{
					Type:    "contract",
					ID:      contract.ID,
					Title:   contract.ContractName,
					Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
				})
			}
		}
	}

	// Measurement / liability / depreciation queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"负债", "折旧", "利息", "摊销", "rou", "计量", "measurement", "depreciation", "liability"}) {
		if req.ContractID != "" {
			results, err := h.mcRepo.GetMeasurementResults(ctx, req.ContractID, "")
			if err == nil && len(results) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", req.ContractID, len(results)))
				for _, r := range results {
					contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
						r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
						r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
				}
				sources = append(sources, Source{
					Type:    "measurement",
					ID:      req.ContractID,
					Title:   "计量结果",
					Snippet: fmt.Sprintf("共 %d 期", len(results)),
				})
			}
		}
	}

	// Journal entry queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"分录", "journal", "会计", "过账", "post", "voucher", "entry", "凭证"}) {
		if req.ContractID != "" {
			entries, err := h.mcRepo.GetJournalEntries(ctx, req.ContractID, "", "")
			if err == nil && len(entries) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", req.ContractID, len(entries)))
				for _, e := range entries {
					desc := ""
					if e.Description != nil {
						desc = *e.Description
					}
					contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
						e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
						e.Amount, e.Currency, e.PostingStatus, desc))
				}
				sources = append(sources, Source{
					Type:    "journal",
					ID:      req.ContractID,
					Title:   "会计分录",
					Snippet: fmt.Sprintf("共 %d 条", len(entries)),
				})
			}
		}
	}

	// Event queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"事件", "变更", "modification", "reassessment", "impairment", "event", "change"}) {
		if req.ContractID != "" {
			events, err := h.eventRepo.GetByContractID(ctx, req.ContractID)
			if err == nil && len(events) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 事件数据（合同 %s，共 %d 条）\n", req.ContractID, len(events)))
				for _, ev := range events {
					reason := ""
					if ev.ChangeReason != nil {
						reason = *ev.ChangeReason
					}
					contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
						ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, reason))
				}
				sources = append(sources, Source{
					Type:    "event",
					ID:      req.ContractID,
					Title:   "变更事件",
					Snippet: fmt.Sprintf("共 %d 条", len(events)),
				})
			}
		}
	}

	// 4. If no context was built and no file was parsed, provide helpful guidance
	if contextData.Len() == 0 {
		contextData.WriteString(`
## 系统信息
当前系统暂无匹配的数据。建议用户：
- 指定具体的合同 ID 以查询计量结果和分录
- 询问合同列表（如"有哪些合同"）
- 上传合同文件或租金表进行 AI 解析
`)
	}

	// 5. Build system prompt
	isWorkingData := false
	if req.PageContext != nil && (req.PageContext.ReportView == "working" || req.PageContext.Page == "monthly-closing") {
		isWorkingData = true
	}
	systemPrompt := h.buildSystemPrompt(userIDStr, roleStr, legalEntityID, contextData.String(), isWorkingData)

	// 6. Call AI Service
	answer, modelName, err := h.callLLM(c, systemPrompt, req.Message, req.History)
	if err != nil {
		// Fallback: return context data without LLM if AI Service is unavailable
		fallbackAnswer := fmt.Sprintf("（AI 服务暂不可用，以下为系统数据摘要）\n\n%s", contextData.String())
		c.JSON(http.StatusOK, AIChatResponse{
			Answer:     fallbackAnswer,
			Sources:    sources,
			Confidence: 0.5,
			IsOfficial: false,
			Model:      "fallback",
		})
		return
	}

	// 7. Extract sources from answer and merge with context sources.
	// If no source citations found in the answer, fall back to all known sources.
	extractedSources := extractSourcesFromAnswer(answer, sources)

	c.JSON(http.StatusOK, AIChatResponse{
		Answer:     answer,
		Sources:    extractedSources,
		Confidence: 0.9,
		IsOfficial: false,
		Model:      modelName,
	})
}

// retrieveContractDetail fetches and appends contract basic info, latest measurements,
// events, and latest journal entries for a given contract ID.
func (h *AIChatHandler) retrieveContractDetail(ctx context.Context, contractID, legalEntityID string, contextData *strings.Builder, sources *[]Source) {
	// Contract basic info
	contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
	if err == nil && contract != nil {
		contextData.WriteString(fmt.Sprintf("\n## 合同详情\n- ID: %s\n- 编号: %s\n- 名称: %s\n- 状态: %s\n- 审批状态: %s\n- 承租方: %s\n- 出租方: %s\n- 门店: %s\n- 币种: %s\n",
			contract.ID, contract.ContractNumber, contract.ContractName,
			contract.ApprovalStatus, contract.Status, contract.LesseeName,
			contract.LessorName, contract.StoreName, contract.Currency))
		if contract.CommencementDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁开始日: %s\n", contract.CommencementDate.Format("2006-01-02")))
		}
		if contract.LeaseEndDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁结束日: %s\n", contract.LeaseEndDate.Format("2006-01-02")))
		}
		if contract.DiscountRateValue != nil {
			contextData.WriteString(fmt.Sprintf("- 折现率: %.4f%%\n", *contract.DiscountRateValue*100))
		}
		if contract.DiscountRateMissing {
			contextData.WriteString("- ⚠️ 折现率缺失，需要人工补充\n")
		}
		*sources = append(*sources, Source{
			Type:    "contract",
			ID:      contract.ID,
			Title:   contract.ContractName,
			Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
		})
	}

	// Latest measurement results
	results, err := h.mcRepo.GetMeasurementResults(ctx, contractID, "")
	if err == nil && len(results) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", contractID, len(results)))
		for _, r := range results {
			contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
				r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
				r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
		}
		*sources = append(*sources, Source{
			Type:    "measurement",
			ID:      contractID,
			Title:   "计量结果",
			Snippet: fmt.Sprintf("合同 %s 共 %d 期", contractID, len(results)),
		})
	}

	// Events for this contract
	events, err := h.eventRepo.GetByContractID(ctx, contractID)
	if err == nil && len(events) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 变更事件（合同 %s，共 %d 条）\n", contractID, len(events)))
		for _, ev := range events {
			reason := ""
			if ev.ChangeReason != nil {
				reason = *ev.ChangeReason
			}
			contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
				ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, reason))
		}
		*sources = append(*sources, Source{
			Type:    "event",
			ID:      contractID,
			Title:   "变更事件",
			Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(events)),
		})
	}

	// Latest journal entries for this contract
	entries, err := h.mcRepo.GetJournalEntries(ctx, contractID, "", "")
	if err == nil && len(entries) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", contractID, len(entries)))
		for _, e := range entries {
			desc := ""
			if e.Description != nil {
				desc = *e.Description
			}
			contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
				e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
				e.Amount, e.Currency, e.PostingStatus, desc))
		}
		*sources = append(*sources, Source{
			Type:    "journal",
			ID:      contractID,
			Title:   "会计分录",
			Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(entries)),
		})
	}
}

// appendReportsContext appends report page context info from the frontend payload.
// It does not query the reports endpoint internally; it uses the provided context only.
func (h *AIChatHandler) appendReportsContext(pc *PageContext, contextData *strings.Builder) {
	contextData.WriteString("\n## 当前报表上下文\n")
	if pc.Title != "" {
		contextData.WriteString(fmt.Sprintf("- 报表标题: %s\n", pc.Title))
	}
	if pc.ReportView != "" {
		contextData.WriteString(fmt.Sprintf("- 报表口径: %s\n", pc.ReportView))
	}
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 覆盖期间: %s\n", pc.Period))
	}
	if len(pc.Filters) > 0 {
		contextData.WriteString("- 筛选条件:\n")
		for k, v := range pc.Filters {
			contextData.WriteString(fmt.Sprintf("  - %s: %s\n", k, v))
		}
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}
}

// appendMonthlyClosingContext appends monthly closing page context and queries
// journal entries for the selected period if provided.
func (h *AIChatHandler) appendMonthlyClosingContext(ctx context.Context, pc *PageContext, contextData *strings.Builder, sources *[]Source) {
	contextData.WriteString("\n## 当前结账中心上下文\n")
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 选定期间: %s\n", pc.Period))
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}

	if pc.Period != "" {
		entries, err := h.mcRepo.GetJournalEntries(ctx, "", pc.Period, "")
		if err == nil && len(entries) > 0 {
			showCount := len(entries)
			if showCount > 20 {
				showCount = 20
			}
			postedCount := 0
			for _, e := range entries {
				if e.PostingStatus == "posted" {
					postedCount++
				}
			}
			contextData.WriteString(fmt.Sprintf("\n### 期间 %s 的会计分录（共 %d 条，显示前 %d 条）\n", pc.Period, len(entries), showCount))
			for i, e := range entries {
				if i >= showCount {
					break
				}
				desc := ""
				if e.Description != nil {
					desc = *e.Description
				}
				contextData.WriteString(fmt.Sprintf("- 合同: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
					e.ContractID, e.EntryType, e.DebitAccount, e.CreditAccount,
					e.Amount, e.Currency, e.PostingStatus, desc))
			}
			contextData.WriteString(fmt.Sprintf("\n汇总: 共 %d 条分录，其中 %d 条已过账\n", len(entries), postedCount))
			*sources = append(*sources, Source{
				Type:    "journal",
				ID:      pc.Period,
				Title:   fmt.Sprintf("期间 %s 会计分录", pc.Period),
				Snippet: fmt.Sprintf("共 %d 条", len(entries)),
			})
		}
	}
}

// buildSystemPrompt constructs the structured system prompt for the LLM.
func (h *AIChatHandler) buildSystemPrompt(userID, role, legalEntityID, contextData string, isWorkingData bool) string {
	workingWarning := ""
	if isWorkingData {
		workingWarning = "\n⚠️ 注意：当前页面展示的是 Working/试算数据，这不是 Official 口径，请在回答中提醒用户。\n"
	}

	return fmt.Sprintf(`你是 IFRS 16 租赁管理系统的 AI 助手。你只能基于以下系统数据回答问题，不能编造信息。

当前用户: %s
角色: %s
可访问法人: %s
%s
%s

请用中文回答，并严格遵循以下结构：

【结论】
用 1-3 句话给出直接回答。

【依据】
逐条列出支撑结论的事实，每条标注来源，格式为：
- ...（来源：合同 LEASE-XXXX-XXXX）
- ...（来源：计量结果 2024-01）
- ...（来源：会计分录 2024-01）

【建议动作】
根据数据和结论，给出具体可执行的建议操作，例如：
- 去补折现率
- 去执行重算
- 去查看摊销报表
- 去审批待处理事件
- 无需操作，数据正常

规则：
- 只能基于提供的系统数据回答，不得编造任何不在上下文中的数字、日期或事实
- 数据不足时明确说"根据当前系统数据，我无法确认"，并说明缺少哪类数据
- 如果是 Working 数据或页面试算数据，要提醒"这不是 Official 口径"
- 建议动作应尽量具体，直接指向系统中的操作页面`, userID, role, legalEntityID, workingWarning, contextData)
}

// callLLM sends the prompt and message history to the AI Service chat endpoint.
func (h *AIChatHandler) callLLM(c *gin.Context, systemPrompt, userMessage string, history []ChatMessage) (string, string, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	// Build messages from history + current message
	messages := []map[string]string{}
	for _, h := range history {
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMessage})

	reqBody, err := json.Marshal(map[string]interface{}{
		"messages":      messages,
		"system_prompt": systemPrompt,
		"temperature":   0.3,
		"max_tokens":    2000,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", aiServiceURL+"/api/v1/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.GetHeader("Authorization"))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("AI service unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result AIChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Answer, result.Model, nil
}

// extractSourcesFromAnswer parses the LLM answer for source citations and matches them
// against the known sources from context retrieval. It returns a deduplicated slice.
func extractSourcesFromAnswer(answer string, knownSources []Source) []Source {
	if len(knownSources) == 0 {
		return knownSources
	}

	// Find source citations in the answer, pattern: （来源：XXX）
	re := regexp.MustCompile(`（来源[：:]\s*([^）)]+)）`)
	matches := re.FindAllStringSubmatch(answer, -1)

	if len(matches) == 0 {
		return knownSources
	}

	// Build a set of cited source tokens
	citedTokens := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			token := strings.TrimSpace(strings.ToLower(m[1]))
			citedTokens[token] = true
		}
	}

	// Find which known sources were cited
	seen := make(map[string]bool)
	var cited []Source
	for _, s := range knownSources {
		// Check if any part of the source (id, title, snippet) matches a citation
		isCited := false
		sourceTokens := strings.ToLower(s.ID + " " + s.Title + " " + s.Snippet)
		for token := range citedTokens {
			if strings.Contains(sourceTokens, token) {
				isCited = true
				break
			}
		}
		if isCited && !seen[s.ID] {
			cited = append(cited, s)
			seen[s.ID] = true
		}
	}

	// If no sources matched citations but citations exist, return known sources
	if len(cited) == 0 && len(matches) > 0 {
		return knownSources
	}
	// If nothing was cited, return known sources (LLM used context data)
	if len(cited) == 0 {
		return knownSources
	}

	return cited
}

func (h *AIChatHandler) parseFile(c *gin.Context, fileID, objectName, contentType string) (map[string]interface{}, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/contract", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.GetHeader("Authorization"))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Extract the extracted_data field
	if data, ok := result["extracted_data"].(map[string]interface{}); ok {
		return data, nil
	}

	return result, nil
}

func containsAny(msg string, targets []string) bool {
	for _, t := range targets {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(t)) {
			return true
		}
	}
	return false
}
