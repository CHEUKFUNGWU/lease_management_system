package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
	"github.com/lease-management-system/core-service/internal/services/draftreview"
)

// DraftReviewUOW narrows the existing draftapp unit of work to the three
// write methods the review workbench needs. draftapp 的 postgres store 在结构
// 上已满足 draftreview.ContractStore，这里只收窄接口，不新建写路径（BG3）。
type DraftReviewUOW struct{ Inner *draftapp.PostgresUnitOfWork }

func (u DraftReviewUOW) Execute(ctx context.Context, fn func(draftreview.ContractStore) error) error {
	if u.Inner == nil {
		return errors.New("draft review unit of work is unavailable")
	}
	return u.Inner.Execute(ctx, func(store draftapp.DraftStore) error { return fn(store) })
}

// ContractDraftReviewHandler serves the /contracts/drafts review face:
// list / detail / revise / decide. 审批复用既有 approval 语义与六角色矩阵，
// 权限点在路由声明处沿用 permission(...)（D-B6/D-B7）。
type ContractDraftReviewHandler struct {
	service *draftreview.Service
}

func NewContractDraftReviewHandler(reader draftreview.Reader, uow draftreview.UnitOfWork) *ContractDraftReviewHandler {
	return &ContractDraftReviewHandler{service: draftreview.NewService(reader, uow)}
}

// reviewerContext 把已认证用户注入 request context；JWT 的 user_id 即
// users.id，UpdateDraftReview 的 reviewed_by 外键因此始终指向真实用户。
func reviewerContext(c *gin.Context) context.Context {
	userID, _ := c.Get("user_id")
	id, _ := userID.(string)
	return draftreview.WithReviewer(c.Request.Context(), id)
}

// ListDrafts GET /contracts/drafts?status=&limit=
func (h *ContractDraftReviewHandler) ListDrafts(c *gin.Context) {
	if h == nil || h.service == nil {
		writeCodedError(c, http.StatusServiceUnavailable, errcontract.CodeDataUnavailable, "draft review service unavailable", nil)
		return
	}
	filter := draftreview.Filter{Status: strings.TrimSpace(c.Query("status"))}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	}
	details, err := h.service.List(reviewerContext(c), filter)
	if err != nil {
		writeCodedFailure(c, http.StatusInternalServerError, err, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": details})
}

// GetDraft GET /contracts/drafts/:id — 异法人与不存在同形：404 + scope_denied
// 码与文案完全一致，无存在性泄漏。
func (h *ContractDraftReviewHandler) GetDraft(c *gin.Context) {
	if h == nil || h.service == nil {
		writeCodedError(c, http.StatusServiceUnavailable, errcontract.CodeDataUnavailable, "draft review service unavailable", nil)
		return
	}
	detail, err := h.service.Get(reviewerContext(c), c.Param("id"))
	if err != nil {
		h.writeReviewError(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// ReviseDraft PUT /contracts/drafts/:id — 字段级修订 + 人工确认标记。
func (h *ContractDraftReviewHandler) ReviseDraft(c *gin.Context) {
	if h == nil || h.service == nil {
		writeCodedError(c, http.StatusServiceUnavailable, errcontract.CodeDataUnavailable, "draft review service unavailable", nil)
		return
	}
	var body struct {
		Edits []draftreview.FieldEdit `json:"edits"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "edits must be an array of {field,value,confirmed}", nil)
		return
	}
	detail, err := h.service.Revise(reviewerContext(c), c.Param("id"), body.Edits)
	if err != nil {
		h.writeReviewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// DecideDrafts POST /contracts/drafts/decide — 批准 / 退回 / 批量批准是同一
// 个端点：单条 = 长度为 1 的列表。响应逐条带结果（部分失败逐条，D-B8）。
func (h *ContractDraftReviewHandler) DecideDrafts(c *gin.Context) {
	if h == nil || h.service == nil {
		writeCodedError(c, http.StatusServiceUnavailable, errcontract.CodeDataUnavailable, "draft review service unavailable", nil)
		return
	}
	var body struct {
		Decisions []draftreview.Decision `json:"decisions"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "decisions must be an array of {draft_id,approve,reason}", nil)
		return
	}
	outcome, err := h.service.Decide(reviewerContext(c), body.Decisions)
	if err != nil {
		writeCodedFailure(c, http.StatusInternalServerError, err, nil)
		return
	}
	c.JSON(http.StatusOK, outcome)
}

// writeReviewError 是 HTTP 侧的错误映射：码与文案一律取自服务层错误对象
// （单一真相源，handler 不手写第二份措辞）；scope_denied 统一 404，使异法
// 人与不存在连状态码都同形。
func (h *ContractDraftReviewHandler) writeReviewError(c *gin.Context, fallbackStatus int, err error) {
	var contractErr *errcontract.Error
	if errors.As(err, &contractErr) {
		status := fallbackStatus
		switch contractErr.Code {
		case errcontract.CodeScopeDenied:
			status = http.StatusNotFound
		case errcontract.CodeBusinessFailure:
			status = http.StatusConflict
		}
		writeCodedError(c, status, contractErr.Code, contractErr.Message, contractErr.Details)
		return
	}
	_ = c.Error(err)
	writeSystemFailure(c, http.StatusInternalServerError, err)
}
