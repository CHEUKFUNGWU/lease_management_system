package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/closecontrol"
)

type CloseExceptionHandler struct {
	service     *closecontrol.Service
	auditLogger *audit.Logger
}

func NewCloseExceptionHandler(service *closecontrol.Service, auditLogger *audit.Logger) *CloseExceptionHandler {
	return &CloseExceptionHandler{service: service, auditLogger: auditLogger}
}

func (h *CloseExceptionHandler) List(c *gin.Context) {
	period := strings.TrimSpace(c.Param("period"))
	if !validAccountingPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会计期间格式应为 YYYY-MM"})
		return
	}
	exceptions, err := h.service.List(c.Request.Context(), period, middleware.GetTenantID(c))
	if err != nil {
		writeCloseExceptionError(c, err)
		return
	}
	scope, scopeAvailable := middleware.GetAccessScope(c)
	scopeComplete := scopeAvailable && (scope.Global ||
		(scope.LegalEntityID != "" && len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0))
	c.JSON(http.StatusOK, gin.H{
		"accounting_period": period,
		"scope_complete":    scopeComplete,
		"data":              exceptions,
		"total":             len(exceptions),
	})
}

func (h *CloseExceptionHandler) Detect(c *gin.Context) {
	period := strings.TrimSpace(c.Param("period"))
	if !validAccountingPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会计期间格式应为 YYYY-MM"})
		return
	}
	if !legalEntityWide(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "正式异常检测需要法人全量权限"})
		return
	}
	if !hasAnyRole(c, "admin", "reviewer", "approver") {
		c.JSON(http.StatusForbidden, gin.H{"error": "当前角色不能运行正式异常检测"})
		return
	}
	result, err := h.service.Detect(c.Request.Context(), closecontrol.DetectCommand{
		AccountingPeriod:  period,
		LegalEntityID:     middleware.GetTenantID(c),
		ProjectionVersion: closecontrol.ProjectionVersion,
		ScopeComplete:     true,
	})
	if err != nil {
		writeCloseExceptionError(c, err)
		return
	}
	uid := currentUserID(c)
	for _, exception := range result.Exceptions {
		if h.auditLogger != nil {
			_ = h.auditLogger.Log(c.Request.Context(), "close_exceptions", exception.ID, "detect", nil, map[string]any{
				"period": period, "fingerprint": exception.Fingerprint, "detection_event_id": exception.DetectionEventID,
			}, uid, c)
		}
	}
	c.JSON(http.StatusOK, result)
}

type closeExceptionActionRequest struct {
	Action  string `json:"action" binding:"required"`
	OwnerID string `json:"owner_id"`
	Note    string `json:"note" binding:"required"`
}

func (h *CloseExceptionHandler) ApplyAction(c *gin.Context) {
	if !legalEntityWide(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "异常治理需要法人全量权限"})
		return
	}
	var req closeExceptionActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 和 note 为必填项"})
		return
	}
	action := closecontrol.Action(strings.TrimSpace(req.Action))
	if !validCloseExceptionAction(action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的异常治理动作"})
		return
	}
	if !roleAllowsAction(c, action) {
		c.JSON(http.StatusForbidden, gin.H{"error": "当前角色不能执行该异常治理动作"})
		return
	}
	uid := currentUserID(c)
	before, after, err := h.service.ApplyAction(c.Request.Context(), closecontrol.ActionCommand{
		ExceptionID: c.Param("id"), Action: action, ActorID: uid, OwnerID: strings.TrimSpace(req.OwnerID), Note: req.Note,
	})
	if err != nil {
		writeCloseExceptionError(c, err)
		return
	}
	if h.auditLogger != nil {
		_ = h.auditLogger.Log(c.Request.Context(), "close_exceptions", after.ID, string(action), before, after, uid, c)
	}
	c.JSON(http.StatusOK, after)
}

func legalEntityWide(c *gin.Context) bool {
	scope, ok := middleware.GetAccessScope(c)
	return ok && (scope.Global || (scope.LegalEntityID != "" && len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0))
}

func hasAnyRole(c *gin.Context, roles ...string) bool {
	value, ok := c.Get("roles")
	if !ok {
		return false
	}
	current, ok := value.([]string)
	if !ok {
		return false
	}
	for _, role := range current {
		for _, wanted := range roles {
			if role == wanted {
				return true
			}
		}
	}
	return false
}

func roleAllowsAction(c *gin.Context, action closecontrol.Action) bool {
	switch action {
	case closecontrol.ActionAssign, closecontrol.ActionVerifyResolution, closecontrol.ActionAccountingConclusion:
		return hasAnyRole(c, "admin", "reviewer")
	case closecontrol.ActionPeriodWaiver, closecontrol.ActionStandingWaiver, closecontrol.ActionClose:
		return hasAnyRole(c, "admin", "approver")
	default:
		return false
	}
}

func currentUserID(c *gin.Context) string {
	value, _ := c.Get("user_id")
	userID, _ := value.(string)
	return userID
}

func validAccountingPeriod(period string) bool {
	_, err := time.Parse("2006-01", period)
	return err == nil
}

func validCloseExceptionAction(action closecontrol.Action) bool {
	switch action {
	case closecontrol.ActionAssign, closecontrol.ActionVerifyResolution, closecontrol.ActionAccountingConclusion,
		closecontrol.ActionPeriodWaiver, closecontrol.ActionStandingWaiver, closecontrol.ActionClose:
		return true
	default:
		return false
	}
}

func writeCloseExceptionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, closecontrol.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "异常不存在或不在当前数据范围"})
	case errors.Is(err, closecontrol.ErrNoteRequired), errors.Is(err, closecontrol.ErrOwnerRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, closecontrol.ErrInvalidTransition), errors.Is(err, closecontrol.ErrRoleSeparation):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
