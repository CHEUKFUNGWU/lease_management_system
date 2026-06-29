package contracts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

type CreateInput struct {
	ContractNumber               string   `json:"contract_number" binding:"required"`
	ContractName                 string   `json:"contract_name" binding:"required"`
	LegalEntityID                *string  `json:"legal_entity_id,omitempty"`
	StoreID                      *string  `json:"store_id,omitempty"`
	LandlordID                   *string  `json:"landlord_id,omitempty"`
	LesseeName                   string   `json:"lessee_name"`
	LessorName                   string   `json:"lessor_name"`
	StoreName                    string   `json:"store_name"`
	StoreAddress                 string   `json:"store_address"`
	Tags                         string   `json:"tags"`
	Currency                     string   `json:"currency" binding:"required"`
	AssetType                    string   `json:"asset_type"`
	CommencementDate             string   `json:"commencement_date" binding:"required"`
	LeaseStartDate               string   `json:"lease_start_date" binding:"required"`
	LeaseEndDate                 string   `json:"lease_end_date" binding:"required"`
	AssetCategory                *string  `json:"asset_category"`
	PropertyCategory             *string  `json:"property_category"`
	SigningDate                  *string  `json:"signing_date"`
	RenewalOptionDescription     *string  `json:"renewal_option_description"`
	TerminationOptionDescription *string  `json:"termination_option_description"`
	RenewalAssessment            *bool    `json:"renewal_assessment"`
	TerminationAssessment        *bool    `json:"termination_assessment"`
	DiscountRateType             *string  `json:"discount_rate_type"`
	DiscountRateVersion          *string  `json:"discount_rate_version"`
	DiscountRateValue            *float64 `json:"discount_rate_value"`
	LeaseScope                   string   `json:"lease_scope"`
	ExemptionReason              *string  `json:"exemption_reason"`
	ScopeSource                  *string  `json:"scope_source"`
	ScopeConfidence              *float64 `json:"scope_confidence"`
}

type UpdateInput struct {
	ContractNumber      string   `json:"contract_number"`
	ContractName        string   `json:"contract_name"`
	LegalEntityID       *string  `json:"legal_entity_id,omitempty"`
	StoreID             *string  `json:"store_id,omitempty"`
	LandlordID          *string  `json:"landlord_id,omitempty"`
	LesseeName          string   `json:"lessee_name"`
	LessorName          string   `json:"lessor_name"`
	StoreName           string   `json:"store_name"`
	StoreAddress        string   `json:"store_address"`
	Tags                string   `json:"tags"`
	Currency            string   `json:"currency"`
	AssetType           string   `json:"asset_type"`
	SigningDate         *string  `json:"signing_date"`
	CommencementDate    string   `json:"commencement_date"`
	LeaseStartDate      string   `json:"lease_start_date"`
	LeaseEndDate        string   `json:"lease_end_date"`
	DiscountRateType    *string  `json:"discount_rate_type"`
	DiscountRateVersion *string  `json:"discount_rate_version"`
	DiscountRateValue   *float64 `json:"discount_rate_value"`
	LeaseScope          string   `json:"lease_scope"`
	ExemptionReason     *string  `json:"exemption_reason"`
	ScopeSource         *string  `json:"scope_source"`
	ScopeConfidence     *float64 `json:"scope_confidence"`
}

type MasterDataResolver interface {
	ResolveLegalEntityID(ctx context.Context, nameOrCode, currency string) (*string, error)
	ResolveOrCreateStoreID(ctx context.Context, name, address string, legalEntityID *string) (*string, error)
	ResolveOrCreateLandlordID(ctx context.Context, name string) (*string, error)
}

func NormalizeTags(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	replacer := strings.NewReplacer(",", " ", "，", " ", ";", " ", "；", " ", "|", " ", "\n", " ", "\t", " ")
	normalized := replacer.Replace(raw)
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return strings.Join(result, ", ")
}

func NormalizeDiscountRateValue(v *float64) *float64 {
	if v == nil {
		return nil
	}
	normalized := *v
	if normalized > 1 {
		normalized = normalized / 100
	}
	return &normalized
}

func NormalizeLeaseScope(scope string) string {
	switch scope {
	case "in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease":
		return scope
	default:
		return "in_scope"
	}
}

func NormalizeAssetType(assetType string) string {
	switch assetType {
	case "real_estate", "vehicle", "it_equipment", "machinery", "other":
		return assetType
	default:
		return "real_estate"
	}
}

func ParseRequiredDate(raw, field string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s format, expected YYYY-MM-DD", field)
	}
	return value, nil
}

func ParseOptionalDate(raw *string, field string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := time.Parse("2006-01-02", *raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s format", field)
	}
	return &value, nil
}

func BuildForCreate(input CreateInput, tenantID string, actorID *string, now time.Time) (*repository.Contract, error) {
	commencementDate, err := ParseRequiredDate(input.CommencementDate, "commencement_date")
	if err != nil {
		return nil, err
	}
	leaseStartDate, err := ParseRequiredDate(input.LeaseStartDate, "lease_start_date")
	if err != nil {
		return nil, err
	}
	leaseEndDate, err := ParseRequiredDate(input.LeaseEndDate, "lease_end_date")
	if err != nil {
		return nil, err
	}
	signingDate, err := ParseOptionalDate(input.SigningDate, "signing_date")
	if err != nil {
		return nil, err
	}
	normalizedDiscountRateValue := NormalizeDiscountRateValue(input.DiscountRateValue)

	contract := &repository.Contract{
		ContractNumber:               input.ContractNumber,
		ContractName:                 input.ContractName,
		Currency:                     input.Currency,
		CommencementDate:             commencementDate,
		LeaseStartDate:               leaseStartDate,
		LeaseEndDate:                 leaseEndDate,
		LesseeName:                   input.LesseeName,
		LessorName:                   input.LessorName,
		StoreName:                    input.StoreName,
		StoreAddress:                 input.StoreAddress,
		Tags:                         NormalizeTags(input.Tags),
		AssetType:                    NormalizeAssetType(input.AssetType),
		AssetCategory:                input.AssetCategory,
		PropertyCategory:             input.PropertyCategory,
		SigningDate:                  signingDate,
		RenewalOptionDescription:     input.RenewalOptionDescription,
		TerminationOptionDescription: input.TerminationOptionDescription,
		RenewalAssessment:            input.RenewalAssessment,
		TerminationAssessment:        input.TerminationAssessment,
		DiscountRateType:             input.DiscountRateType,
		DiscountRateVersion:          input.DiscountRateVersion,
		DiscountRateValue:            normalizedDiscountRateValue,
		DiscountRateMissing:          normalizedDiscountRateValue == nil,
		LeaseScope:                   NormalizeLeaseScope(input.LeaseScope),
		ExemptionReason:              input.ExemptionReason,
		ScopeSource:                  input.ScopeSource,
		ScopeConfidence:              input.ScopeConfidence,
		CreatedBy:                    actorID,
	}

	applyTenantAndMasterDataHints(contract, input.LegalEntityID, input.StoreID, input.LandlordID, tenantID)
	applyScopeClassificationMetadata(contract, actorID, now)

	return contract, nil
}

func BuildForUpdate(id string, existing *repository.Contract, input UpdateInput, actorID *string, now time.Time) (*repository.Contract, error) {
	commencementDate, err := ParseRequiredDate(input.CommencementDate, "commencement_date")
	if err != nil {
		return nil, err
	}
	leaseStartDate, err := ParseRequiredDate(input.LeaseStartDate, "lease_start_date")
	if err != nil {
		return nil, err
	}
	leaseEndDate, err := ParseRequiredDate(input.LeaseEndDate, "lease_end_date")
	if err != nil {
		return nil, err
	}
	signingDate, err := ParseOptionalDate(input.SigningDate, "signing_date")
	if err != nil {
		return nil, err
	}
	normalizedDiscountRateValue := NormalizeDiscountRateValue(input.DiscountRateValue)

	leaseScope := existing.LeaseScope
	if input.LeaseScope != "" {
		leaseScope = NormalizeLeaseScope(input.LeaseScope)
	}
	assetType := existing.AssetType
	if input.AssetType != "" {
		assetType = NormalizeAssetType(input.AssetType)
	}

	contract := &repository.Contract{
		ID:                  id,
		ContractNumber:      input.ContractNumber,
		ContractName:        input.ContractName,
		LegalEntityID:       input.LegalEntityID,
		StoreID:             input.StoreID,
		LandlordID:          input.LandlordID,
		LesseeName:          input.LesseeName,
		LessorName:          input.LessorName,
		StoreName:           input.StoreName,
		StoreAddress:        input.StoreAddress,
		Tags:                NormalizeTags(input.Tags),
		Currency:            input.Currency,
		AssetType:           assetType,
		SigningDate:         signingDate,
		CommencementDate:    commencementDate,
		LeaseStartDate:      leaseStartDate,
		LeaseEndDate:        leaseEndDate,
		DiscountRateType:    input.DiscountRateType,
		DiscountRateVersion: input.DiscountRateVersion,
		DiscountRateValue:   normalizedDiscountRateValue,
		DiscountRateMissing: normalizedDiscountRateValue == nil,
		LeaseScope:          leaseScope,
		ExemptionReason:     input.ExemptionReason,
		ScopeSource:         input.ScopeSource,
		ScopeConfidence:     input.ScopeConfidence,
		Status:              existing.Status,
	}

	applyScopeClassificationMetadata(contract, actorID, now)

	return contract, nil
}

func ResolveMasterData(ctx context.Context, resolver MasterDataResolver, contract *repository.Contract, legalEntityHint string) error {
	if contract.LegalEntityID == nil || *contract.LegalEntityID == "" {
		resolved, err := resolver.ResolveLegalEntityID(ctx, strings.TrimSpace(legalEntityHint), contract.Currency)
		if err != nil {
			return err
		}
		contract.LegalEntityID = resolved
	}

	if contract.StoreID == nil || *contract.StoreID == "" {
		resolved, err := resolver.ResolveOrCreateStoreID(ctx, strings.TrimSpace(contract.StoreName), strings.TrimSpace(contract.StoreAddress), contract.LegalEntityID)
		if err != nil {
			return err
		}
		contract.StoreID = resolved
	}

	if contract.LandlordID == nil || *contract.LandlordID == "" {
		resolved, err := resolver.ResolveOrCreateLandlordID(ctx, strings.TrimSpace(contract.LessorName))
		if err != nil {
			return err
		}
		contract.LandlordID = resolved
	}

	return nil
}

func applyTenantAndMasterDataHints(contract *repository.Contract, legalEntityID, storeID, landlordID *string, tenantID string) {
	if legalEntityID != nil && *legalEntityID != "" {
		contract.LegalEntityID = legalEntityID
	} else if tenantID != "" {
		contract.LegalEntityID = &tenantID
	}
	if storeID != nil && *storeID != "" {
		contract.StoreID = storeID
	}
	if landlordID != nil && *landlordID != "" {
		contract.LandlordID = landlordID
	}
}

func applyScopeClassificationMetadata(contract *repository.Contract, actorID *string, now time.Time) {
	if contract.ScopeSource == nil {
		source := "manual"
		contract.ScopeSource = &source
	}
	if actorID != nil && *actorID != "" {
		contract.ScopeClassifiedBy = actorID
		contract.ScopeClassifiedAt = &now
	}
}
