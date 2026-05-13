package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type ReportHandler struct {
	contractRepo *repository.ContractRepository
	psRepo       *repository.PaymentScheduleRepository
}

func NewReportHandler(contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository) *ReportHandler {
	return &ReportHandler{
		contractRepo: contractRepo,
		psRepo:       psRepo,
	}
}

// LiabilityRolling returns lease liability rolling schedule
// Query param: mode=working|official
func (h *ReportHandler) LiabilityRolling(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	legalEntityID := middleware.GetTenantID(c)
	
	// Build status filter based on report mode
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		// Working report includes all non-rejected statuses
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}
	
	// Query contracts with status filter and tenant isolation
	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	var results []gin.H
	for _, contract := range contracts {
		results = append(results, gin.H{
			"contract_id":       contract.ID,
			"contract_number":   contract.ContractNumber,
			"contract_name":     contract.ContractName,
			"approval_status":   contract.ApprovalStatus,
			"is_official_version": contract.IsOfficialVersion,
			"report_mode":       contract.ReportMode,
			"commencement_date": contract.CommencementDate.Format("2006-01-02"),
			"lease_end_date":    contract.LeaseEndDate.Format("2006-01-02"),
			"currency":          contract.Currency,
			"discount_rate_type": contract.DiscountRateType,
			"discount_rate_missing": contract.DiscountRateMissing,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"mode":     mode,
		"is_official": mode == "official",
		"data":     results,
		"total":    len(results),
		"generated_at": time.Now(),
	})
}

// ContractSummary returns summary statistics for the dashboard
func (h *ReportHandler) ContractSummary(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	legalEntityID := middleware.GetTenantID(c)
	
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}
	
	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	var totalContracts, approvedContracts, draftContracts int
	for _, c := range contracts {
		totalContracts++
		switch c.ApprovalStatus {
		case "approved":
			approvedContracts++
		case "draft":
			draftContracts++
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"mode":              mode,
		"total_contracts":   totalContracts,
		"approved_count":    approvedContracts,
		"draft_count":       draftContracts,
		"pending_count":     totalContracts - approvedContracts - draftContracts,
	})
}

// ExportLiabilityRolling exports contract data as CSV
func (h *ReportHandler) ExportLiabilityRolling(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}
	
	legalEntityID := middleware.GetTenantID(c)
	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Set headers for CSV download
	marker := ""
	if mode == "working" {
		marker = "_WORKING_DRAFT"
	} else {
		marker = "_OFFICIAL"
	}
	
	filename := fmt.Sprintf("IFRS16_LiabilityRolling%s_%s.csv", marker, time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	
	// BOM for Excel UTF-8
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	
	// Header
	writer.Write([]string{
		"合同编号", "合同名称", "审批状态", "是否正式版", "报表模式",
		"法人主体ID", "门店ID", "出租方ID", "币种",
		"租赁起始日", "租赁结束日", "折现率类型", "折现率缺失",
		"创建时间",
	})
	
	// Data rows
	for _, contract := range contracts {
		writer.Write([]string{
			contract.ContractNumber,
			contract.ContractName,
			contract.ApprovalStatus,
			fmt.Sprintf("%v", contract.IsOfficialVersion),
			contract.ReportMode,
			contract.LegalEntityID,
			contract.StoreID,
			contract.LandlordID,
			contract.Currency,
			contract.CommencementDate.Format("2006-01-02"),
			contract.LeaseEndDate.Format("2006-01-02"),
			func() string {
				if contract.DiscountRateType != nil {
					return *contract.DiscountRateType
				}
				return ""
			}(),
			fmt.Sprintf("%v", contract.DiscountRateMissing),
			contract.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}
