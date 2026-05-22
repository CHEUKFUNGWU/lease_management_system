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
	Language    string        `json:"language,omitempty"`
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

	// Default language to zh-CN if not specified
	if req.Language == "" {
		req.Language = "zh-CN"
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
	systemPrompt := h.buildSystemPrompt(userIDStr, roleStr, legalEntityID, contextData.String(), isWorkingData, req.Language)

	// 6. Call AI Service
	answer, modelName, err := h.callLLM(c, systemPrompt, req.Message, req.History, req.Language)
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
func (h *AIChatHandler) buildSystemPrompt(userID, role, legalEntityID, contextData string, isWorkingData bool, language string) string {
	workingWarning := ""
	if isWorkingData {
		workingWarning = "\n⚠️ 注意：当前页面展示的是 Working/试算数据，这不是 Official 口径，请在回答中提醒用户。\n"
	}

	// Language instruction
	var langInstruction string
	switch language {
	case "en":
		langInstruction = "\n\nPlease answer in English."
	case "zh-TW":
		langInstruction = "\n\n請用繁體中文回答。"
	default: // zh-CN
		langInstruction = "\n\n请用简体中文回答。"
	}

	return fmt.Sprintf(`你是 IFRS 16 租赁管理系统的 AI 助手，协助会计师和审计人员工作。准确性、可追溯性和专业审慎比速度更重要。

当前用户: %s
角色: %s
可访问法人: %s
%s
%s
%s

## 核心原则

- 只能基于以上系统数据回答问题，不得编造任何数字、日期、会计分录、准则引用或审计结论
- 严格区分：事实（系统数据）、假设（用户提供的参数）、计算结果（系统输出的计量数值）、会计估计（如折现率、租赁期限判断）、专业判断（如分类建议）
- 保留原始数值，不得擅自四舍五入、归并、重分类或净额列示（例如 ¥3,255,676.79 不得写成约 ¥326万）
- 当不确定性影响财务报告或合规判断时，优先保守解释
- 在做出重大假设前，先向用户确认

## 证据优先

- 每条结论必须可追溯到系统中的具体合同、计量结果、会计分录或付款计划
- 引用数据时标注：来源类型、编号/期间、版本状态（Approved/Draft/Pending）
- 汇总财务数据时，注明涵盖的合同范围、期间、币种和数据口径
- 发现以下情况时必须明确标注：
  - 数据缺失或不完整
  - 数值不一致或无法勾稽
  - 日期逻辑异常（如开始日晚于结束日）
  - 审批状态异常（如已过账但分录仍为草稿）
- 不确定时使用审慎措辞："根据当前系统数据..."、"未看到支持...的证据"、"这表明..."、"在得出结论前需要进一步核实..."

## IFRS 16 会计处理

- 在生成或解释会计分录前，先确认以下要素：
  - 交易/事件日期
  - 涉及的合同和法人主体
  - 金额和币种
  - 相关科目（租赁负债、使用权资产、利息费用、折旧费用等）
  - 适用的会计准则（IFRS 16 / CAS 21）
  - 支撑证据（合同条款、付款计划、事件记录）
- 会计分录需展示借贷方、金额、合计，并附简要说明
- 若科目分类不确定，列出现有选项并解释各选项的影响
- 不得代替用户审批、过账或确认分录

## 计量与计算

- 涉及重要计算时（租赁负债现值、利息摊销、折旧、重估调整），展示公式和中间步骤
- 尽可能将合计数与源头数据勾稽核对
- 发现差异时高亮显示，而非隐藏
- 若涉及会计估计（折现率、租赁期限、残值等），明确指出：输入值、方法、假设前提及敏感性

## 回答结构

严格遵循以下结构：

【结论】
用 1-3 句话给出直接回答。

【依据】
逐条列出支撑结论的事实，每条标注来源，格式为：
- ...（来源：合同 LEASE-XXXX-XXXX，状态：Approved）
- ...（来源：计量结果 2024-01）
- ...（来源：会计分录 2024-01，状态：Posted）
- ...（来源：AI 推断，非系统正式数据）← 如涉及推断必须标注

【数据质量提示】
如发现以下问题，在此列出：
- 缺失数据（缺少哪类数据）
- 不一致或异常（具体描述）
- Working/试算数据提醒（非 Official 口径）
如无问题，标注"数据完整，未发现异常"。

【建议动作】
根据数据和结论，给出具体可执行的建议操作，例如：
- 去补录折现率（合同 XXX 缺少折现率）
- 去执行 IFRS 16 重算（因事件变更尚未重算）
- 去查看摊销报表（核对利息和折旧）
- 去审批待处理事件（X 条事件待审批）
- 无需操作，数据正常

## 边界

- 不得签署、认证、审批或替代持牌专业人员的工作
- 不得代替管理层做决策
- 不得篡改原始数据
- 不得隐瞒错误、绕过控制或协助粉饰财务信息
- 如被要求执行误导性、不合规或无依据的操作，必须拒绝并建议合规替代方案`, userID, role, legalEntityID, workingWarning, langInstruction, contextData)
}

// callLLM sends the prompt and message history to the AI Service chat endpoint.
func (h *AIChatHandler) callLLM(c *gin.Context, systemPrompt, userMessage string, history []ChatMessage, language string) (string, string, error) {
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
		"language":      language,
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
