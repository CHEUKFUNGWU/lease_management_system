package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ContractRequest struct {
	ContractNumber   string `json:"contract_number" binding:"required"`
	ContractName     string `json:"contract_name" binding:"required"`
	LegalEntityID    string `json:"legal_entity_id" binding:"required,uuid"`
	StoreID          string `json:"store_id" binding:"required,uuid"`
	LandlordID       string `json:"landlord_id" binding:"required,uuid"`
	Currency         string `json:"currency" binding:"required"`
	CommencementDate string `json:"commencement_date" binding:"required"`
	LeaseStartDate   string `json:"lease_start_date" binding:"required"`
	LeaseEndDate     string `json:"lease_end_date" binding:"required"`
}

type Contract struct {
	ID               string    `json:"id"`
	ContractNumber   string    `json:"contract_number"`
	ContractName     string    `json:"contract_name"`
	LegalEntityID    string    `json:"legal_entity_id"`
	StoreID          string    `json:"store_id"`
	LandlordID       string    `json:"landlord_id"`
	Currency         string    `json:"currency"`
	CommencementDate string    `json:"commencement_date"`
	LeaseStartDate   string    `json:"lease_start_date"`
	LeaseEndDate     string    `json:"lease_end_date"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// In-memory contract store for MVP
var contracts = make(map[string]*Contract)

func CreateContract() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ContractRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		contract := &Contract{
			ID:               uuid.New().String(),
			ContractNumber:   req.ContractNumber,
			ContractName:     req.ContractName,
			LegalEntityID:    req.LegalEntityID,
			StoreID:          req.StoreID,
			LandlordID:       req.LandlordID,
			Currency:         req.Currency,
			CommencementDate: req.CommencementDate,
			LeaseStartDate:   req.LeaseStartDate,
			LeaseEndDate:     req.LeaseEndDate,
			Status:           "draft",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		contracts[contract.ID] = contract
		
		c.JSON(http.StatusCreated, contract)
	}
}

func GetContracts() gin.HandlerFunc {
	return func(c *gin.Context) {
		list := make([]*Contract, 0, len(contracts))
		for _, c := range contracts {
			list = append(list, c)
		}
		c.JSON(http.StatusOK, gin.H{"data": list, "total": len(list)})
	}
}

func GetContract() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		contract, exists := contracts[id]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
			return
		}
		c.JSON(http.StatusOK, contract)
	}
}
