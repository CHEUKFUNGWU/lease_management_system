package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/newstorefeasibility"
)

// RH4 新店可行性端点（R2-2）。
//
// POST /api/v1/retail/new-store-feasibility
//
// 租赁投影端口按请求租户绑定构造（底线 1）：法人 B 的请求即使携带
// 法人 A 的 contract_id，投影查询也以 B 的 legal_entity_id 过滤，
// 得不到任何计量行——端口未接线语义接管，返回具名 Gap。
// 本端点纯计算不落库。

type NewStoreFeasibilityHandler struct {
	db repository.DBTX
}

func NewNewStoreFeasibilityHandler(db repository.DBTX) *NewStoreFeasibilityHandler {
	return &NewStoreFeasibilityHandler{db: db}
}

func (h *NewStoreFeasibilityHandler) Evaluate(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	if h.db == nil {
		writeCodedError(c, http.StatusServiceUnavailable, errcontract.CodeDataUnavailable, "repository unavailable", nil)
		return
	}

	var in newstorefeasibility.Input
	if err := decodeStrictJSON(c, &in); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	// horizon 上限保护：60 个月足够可行性测算，防滥用长循环
	if in.Horizon > 60 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "horizon must be ≤ 60 months", nil)
		return
	}
	if in.Currency == "" {
		in.Currency = "CNY"
	}

	ports := newstorefeasibility.Ports{
		LeaseProjection: repository.NewTenantBoundLeaseProjection(h.db, legalEntityID),
	}
	res := newstorefeasibility.Evaluate(c.Request.Context(), in, ports)
	c.JSON(http.StatusOK, res)
}
