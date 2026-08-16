package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
)

// ErrRetailStoreDayFactIdempotencyConflict is returned when an idempotency key
// is reused by the same scope with a different payload.
var ErrRetailStoreDayFactIdempotencyConflict = errors.New("retail store-day fact idempotency key conflicts with a different payload")

// RetailStoreDayFact is the source-grain daily fact used by the retail
// workstation. It intentionally does not replace StoreOperatingFact, whose
// monthly contract is consumed by existing FP&A features.
type RetailStoreDayFact struct {
	ID                       string    `json:"id"`
	StoreID                  string    `json:"store_id"`
	StoreCode                string    `json:"store_code"`
	StoreName                string    `json:"store_name"`
	Brand                    string    `json:"brand"`
	Region                   string    `json:"region"`
	BusinessDate             string    `json:"business_date"`
	Currency                 string    `json:"currency"`
	Revenue                  float64   `json:"revenue"`
	GrossProfit              *float64  `json:"gross_profit,omitempty"`
	Transactions             *float64  `json:"transactions,omitempty"`
	Footfall                 *float64  `json:"footfall,omitempty"`
	AreaSqm                  *float64  `json:"area_sqm,omitempty"`
	LaborCost                *float64  `json:"labor_cost,omitempty"`
	FixedRent                *float64  `json:"fixed_rent,omitempty"`
	VariableRent             *float64  `json:"variable_rent,omitempty"`
	NonLeaseCost             *float64  `json:"non_lease_cost,omitempty"`
	OtherControllableCost    *float64  `json:"other_controllable_cost,omitempty"`
	SourceSystem             string    `json:"source_system"`
	SourceRecordID           string    `json:"source_record_id,omitempty"`
	ImportBatchID            *string   `json:"import_batch_id,omitempty"`
	AsOfAt                   time.Time `json:"as_of_at"`
	Version                  int       `json:"version"`
	ReconciliationStatus     string    `json:"reconciliation_status"`
	MappingStatus            string    `json:"mapping_status"`
	DataQualityStatus        string    `json:"data_quality_status"`
	DataClassification       string    `json:"data_classification"`
	SimulationDatasetVersion *string   `json:"simulation_dataset_version,omitempty"`
	CreatedBy                *string   `json:"created_by,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// RetailStoreDayFactsPage carries a reliable total alongside a bounded page.
// A zero PageSize means no limit and is used only by the backwards-compatible
// repository method below.
type RetailStoreDayFactsPage struct {
	Data     []*RetailStoreDayFact
	Total    int
	PageSize int
	Offset   int
	Returned int
}

// RetailStoreDayFactWriteResult is the outcome of one transactional request.
type RetailStoreDayFactWriteResult struct {
	Facts    []*RetailStoreDayFact
	Replayed bool
}

// RetailStoreDayFactAuditFunc writes the audit row through the supplied
// transaction. Returning an error causes the fact transaction to roll back.
type RetailStoreDayFactAuditFunc func(context.Context, DBTX, *RetailStoreDayFact, *RetailStoreDayFact) error

const retailStoreDayFactColumns = `f.id,f.store_id,s.code,s.name,COALESCE(s.brand,''),COALESCE(s.region,''),
	f.business_date::text,f.currency,f.revenue,f.gross_profit,f.transactions,f.footfall,f.area_sqm,
	f.labor_cost,f.fixed_rent,f.variable_rent,f.non_lease_cost,f.other_controllable_cost,
	f.source_system,COALESCE(f.source_record_id,''),f.import_batch_id,f.as_of_at,f.version,
	f.reconciliation_status,f.mapping_status,f.data_quality_status,f.data_classification,
	f.simulation_dataset_version,f.created_by,f.created_at,f.updated_at`

// UpsertRetailStoreDayFact resolves the store through the caller's tenant and
// data scope before writing. It also verifies that an import batch belongs to
// the same legal entity as the store. The database uniqueness key makes a
// business-key replay deterministic.
func (r *OperatingFactsRepository) UpsertRetailStoreDayFact(ctx context.Context, entity access.EntityFilter, fact *RetailStoreDayFact) (*RetailStoreDayFact, error) {
	if fact == nil {
		return nil, fmt.Errorf("upsert retail store-day fact: fact is nil")
	}
	r.prepareRetailStoreDayFact(fact)
	resolvedLegalEntity, err := r.resolveRetailStoreDayFactStore(ctx, entity, fact)
	if err != nil {
		return nil, err
	}
	if err := r.validateRetailStoreDayFactImportBatch(ctx, fact.ImportBatchID, resolvedLegalEntity); err != nil {
		return nil, err
	}

	query := `
		INSERT INTO retail_store_day_facts (
			id,store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,
			fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,source_record_id,
			import_batch_id,as_of_at,version,reconciliation_status,mapping_status,data_quality_status,
			data_classification,simulation_dataset_version,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)
		ON CONFLICT (store_id,business_date,version,source_system) DO UPDATE SET
			currency=EXCLUDED.currency,revenue=EXCLUDED.revenue,gross_profit=EXCLUDED.gross_profit,
			transactions=EXCLUDED.transactions,footfall=EXCLUDED.footfall,area_sqm=EXCLUDED.area_sqm,
			labor_cost=EXCLUDED.labor_cost,fixed_rent=EXCLUDED.fixed_rent,variable_rent=EXCLUDED.variable_rent,
			non_lease_cost=EXCLUDED.non_lease_cost,other_controllable_cost=EXCLUDED.other_controllable_cost,
			source_record_id=EXCLUDED.source_record_id,import_batch_id=EXCLUDED.import_batch_id,
			as_of_at=EXCLUDED.as_of_at,reconciliation_status=EXCLUDED.reconciliation_status,
			mapping_status=EXCLUDED.mapping_status,data_quality_status=EXCLUDED.data_quality_status,
			data_classification=EXCLUDED.data_classification,
			simulation_dataset_version=EXCLUDED.simulation_dataset_version,updated_at=NOW()
		RETURNING id,created_by,created_at,updated_at`
	if err := r.db.QueryRow(ctx, query,
		fact.ID, fact.StoreID, fact.BusinessDate, fact.Currency, fact.Revenue, fact.GrossProfit,
		fact.Transactions, fact.Footfall, fact.AreaSqm, fact.LaborCost, fact.FixedRent, fact.VariableRent,
		fact.NonLeaseCost, fact.OtherControllableCost, fact.SourceSystem, nullableText(fact.SourceRecordID),
		fact.ImportBatchID, fact.AsOfAt, fact.Version, fact.ReconciliationStatus, fact.MappingStatus,
		fact.DataQualityStatus, fact.DataClassification, fact.SimulationDatasetVersion, fact.CreatedBy,
	).Scan(&fact.ID, &fact.CreatedBy, &fact.CreatedAt, &fact.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert retail store-day fact for legal entity %s: %w", resolvedLegalEntity, err)
	}
	return fact, nil
}

// UpsertRetailStoreDayFactsAtomic commits all facts and all corresponding
// audit callbacks together. It also persists request-level idempotency. A
// replay loads the previously written fact IDs and never calls the upsert or
// audit callback again.
func (r *OperatingFactsRepository) UpsertRetailStoreDayFactsAtomic(
	ctx context.Context,
	entity access.EntityFilter,
	facts []*RetailStoreDayFact,
	idempotencyKey, payloadSHA256 string,
	createdBy *string,
	auditFn RetailStoreDayFactAuditFunc,
) (*RetailStoreDayFactWriteResult, error) {
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("retail store-day fact transaction requires a PostgreSQL pool")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin retail store-day fact transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	txRepo := NewOperatingFactsRepository(tx)
	scopeKey := "global"
	if entityID, idErr := entity.LegalEntityID(); idErr == nil {
		scopeKey = entityID
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		if strings.TrimSpace(payloadSHA256) == "" {
			return nil, fmt.Errorf("idempotency payload hash is required")
		}
		var requestID string
		var legalEntityArg any
		if !entity.IsGlobal() {
			scopedID, _ := entity.LegalEntityID()
			legalEntityArg = scopedID
		}
		insertErr := tx.QueryRow(ctx, `
			INSERT INTO retail_store_day_fact_requests (scope_key,legal_entity_id,idempotency_key,payload_sha256,created_by)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (scope_key,idempotency_key) DO NOTHING
			RETURNING id`, scopeKey, legalEntityArg, strings.TrimSpace(idempotencyKey), strings.TrimSpace(payloadSHA256), createdBy).Scan(&requestID)
		if insertErr == pgx.ErrNoRows {
			var storedHash string
			var rawFactIDs []byte
			if err := tx.QueryRow(ctx, `
				SELECT payload_sha256,fact_ids
				FROM retail_store_day_fact_requests
				WHERE scope_key=$1 AND idempotency_key=$2`, scopeKey, strings.TrimSpace(idempotencyKey)).Scan(&storedHash, &rawFactIDs); err != nil {
				return nil, fmt.Errorf("load retail store-day idempotency record: %w", err)
			}
			if storedHash != strings.TrimSpace(payloadSHA256) {
				return nil, fmt.Errorf("%w: scope=%s key=%s", ErrRetailStoreDayFactIdempotencyConflict, scopeKey, strings.TrimSpace(idempotencyKey))
			}
			var factIDs []string
			if err := json.Unmarshal(rawFactIDs, &factIDs); err != nil {
				return nil, fmt.Errorf("decode retail store-day idempotency fact ids: %w", err)
			}
			loaded, err := txRepo.listRetailStoreDayFactsByIDs(ctx, entity, factIDs)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit retail store-day idempotent replay: %w", err)
			}
			committed = true
			return &RetailStoreDayFactWriteResult{Facts: loaded, Replayed: true}, nil
		}
		if insertErr != nil {
			return nil, fmt.Errorf("persist retail store-day idempotency record: %w", insertErr)
		}
		_ = requestID
	}

	saved := make([]*RetailStoreDayFact, 0, len(facts))
	factIDs := make([]string, 0, len(facts))
	for index, fact := range facts {
		if fact == nil {
			return nil, fmt.Errorf("retail store-day fact at index %d is nil", index)
		}
		txRepo.prepareRetailStoreDayFact(fact)
		old, err := txRepo.getRetailStoreDayFactByBusinessKey(ctx, entity, fact)
		if err != nil {
			return nil, fmt.Errorf("load old retail store-day fact at index %d: %w", index, err)
		}
		result, err := txRepo.UpsertRetailStoreDayFact(ctx, entity, fact)
		if err != nil {
			return nil, fmt.Errorf("upsert retail store-day fact at index %d: %w", index, err)
		}
		if auditFn != nil {
			if err := auditFn(ctx, tx, old, result); err != nil {
				return nil, fmt.Errorf("audit retail store-day fact at index %d: %w", index, err)
			}
		}
		saved = append(saved, result)
		factIDs = append(factIDs, result.ID)
	}

	if strings.TrimSpace(idempotencyKey) != "" {
		rawFactIDs, err := json.Marshal(factIDs)
		if err != nil {
			return nil, fmt.Errorf("encode retail store-day idempotency fact ids: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE retail_store_day_fact_requests
			SET fact_ids=$2::jsonb
			WHERE scope_key=$1 AND idempotency_key=$3`, scopeKey, rawFactIDs, strings.TrimSpace(idempotencyKey)); err != nil {
			return nil, fmt.Errorf("save retail store-day idempotency result: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit retail store-day fact transaction: %w", err)
	}
	committed = true
	return &RetailStoreDayFactWriteResult{Facts: saved}, nil
}

// ListRetailStoreDayFactsPage returns a bounded page and a reliable total.
func (r *OperatingFactsRepository) ListRetailStoreDayFactsPage(ctx context.Context, entity access.EntityFilter, dateFrom, dateTo string, storeIDs []string, pageSize, offset int) (*RetailStoreDayFactsPage, error) {
	if pageSize < 0 || offset < 0 {
		return nil, fmt.Errorf("retail store-day page size and offset must not be negative")
	}
	fromWhere, args, nextArg := retailStoreDayFactFilter(ctx, entity, dateFrom, dateTo, storeIDs)
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*)"+fromWhere, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count retail store-day facts: %w", err)
	}
	dataQuery := "SELECT " + retailStoreDayFactColumns + fromWhere + " ORDER BY f.business_date,s.code,f.version DESC"
	dataArgs := append([]any(nil), args...)
	if pageSize > 0 {
		dataQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", nextArg, nextArg+1)
		dataArgs = append(dataArgs, pageSize, offset)
	}
	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("list retail store-day facts page: %w", err)
	}
	defer rows.Close()
	result := make([]*RetailStoreDayFact, 0)
	for rows.Next() {
		item, err := scanRetailStoreDayFact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan retail store-day fact page: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retail store-day fact page: %w", err)
	}
	return &RetailStoreDayFactsPage{Data: result, Total: total, PageSize: pageSize, Offset: offset, Returned: len(result)}, nil
}

// ListRetailStoreDayFacts remains source-compatible with the first MAX-001
// implementation but no longer silently truncates at 50,000 rows.
func (r *OperatingFactsRepository) ListRetailStoreDayFacts(ctx context.Context, entity access.EntityFilter, dateFrom, dateTo string, storeIDs []string) ([]*RetailStoreDayFact, error) {
	page, err := r.ListRetailStoreDayFactsPage(ctx, entity, dateFrom, dateTo, storeIDs, 0, 0)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

// RetailStoreDayExistingState returns the current max fact version per
// "storeID|businessDate" key for one source system's production facts; pairs
// with no row simply stay absent. This is the read side of the M8 correction
// policy: a re-imported (store, date) gets max+1 and supersedes, history
// stays queryable.
func (r *OperatingFactsRepository) RetailStoreDayExistingState(ctx context.Context, storeIDs, businessDates []string, sourceSystem string) (map[string]int, error) {
	if len(storeIDs) == 0 || len(businessDates) == 0 {
		return map[string]int{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT s.id::text, d.day::text, COALESCE(MAX(f.version), 0)
		FROM unnest($1::uuid[]) AS s(id)
		CROSS JOIN unnest($2::date[]) AS d(day)
		LEFT JOIN retail_store_day_facts f
			ON f.store_id = s.id AND f.business_date = d.day
			AND f.source_system = $3 AND f.data_classification = 'production'
		GROUP BY s.id, d.day`, storeIDs, businessDates, sourceSystem)
	if err != nil {
		return nil, fmt.Errorf("query retail store-day existing state: %w", err)
	}
	defer rows.Close()
	state := map[string]int{}
	for rows.Next() {
		var storeID, businessDate string
		var version int
		if err := rows.Scan(&storeID, &businessDate, &version); err != nil {
			return nil, fmt.Errorf("scan retail store-day existing state: %w", err)
		}
		state[storeID+"|"+businessDate] = version
	}
	return state, rows.Err()
}

func (r *OperatingFactsRepository) prepareRetailStoreDayFact(fact *RetailStoreDayFact) {
	if fact.ID == "" {
		fact.ID = uuid.NewString()
	}
	if fact.Version <= 0 {
		fact.Version = 1
	}
	if fact.AsOfAt.IsZero() {
		fact.AsOfAt = time.Now().UTC()
	}
	if fact.ReconciliationStatus == "" {
		fact.ReconciliationStatus = "unreconciled"
	}
	if fact.MappingStatus == "" {
		fact.MappingStatus = "mapped"
	}
	if fact.DataQualityStatus == "" {
		fact.DataQualityStatus = "unassessed"
	}
}

func (r *OperatingFactsRepository) resolveRetailStoreDayFactStore(ctx context.Context, entity access.EntityFilter, fact *RetailStoreDayFact) (string, error) {
	storeQuery := `SELECT s.legal_entity_id::text,s.code,s.name,COALESCE(s.brand,''),COALESCE(s.region,'')
		FROM stores s WHERE s.id=$1`
	storeArgs := []any{fact.StoreID}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(storeArgs)+1); err != nil {
		return "", err
	} else if clause != "" {
		storeQuery += " AND " + clause
		storeArgs = append(storeArgs, arg)
	}
	storeQuery, storeArgs, _ = appendStoreScopePredicate(ctx, storeQuery, storeArgs, len(storeArgs)+1, "s")
	var resolvedLegalEntity string
	if err := r.db.QueryRow(ctx, storeQuery, storeArgs...).Scan(&resolvedLegalEntity, &fact.StoreCode, &fact.StoreName, &fact.Brand, &fact.Region); err != nil {
		if err == pgx.ErrNoRows {
			// The store is either missing or outside the caller's tenant
			// scope. AGENTS.md forbids softening a scope refusal into "no
			// data", and the Agent seam already treats a store that is not
			// visible under the caller scope as scope_denied — so this write
			// path says so explicitly instead of hiding the reason.
			return "", errcontract.New(errcontract.CodeScopeDenied, "retail store is outside the caller data scope")
		}
		return "", fmt.Errorf("resolve retail store-day fact store: %w", err)
	}
	return resolvedLegalEntity, nil
}

func (r *OperatingFactsRepository) validateRetailStoreDayFactImportBatch(ctx context.Context, importBatchID *string, storeLegalEntity string) error {
	if importBatchID == nil || strings.TrimSpace(*importBatchID) == "" {
		return nil
	}
	var batchLegalEntity *string
	if err := r.db.QueryRow(ctx, `SELECT legal_entity_id::text FROM operating_fact_batches WHERE id=$1`, strings.TrimSpace(*importBatchID)).Scan(&batchLegalEntity); err != nil {
		if err == pgx.ErrNoRows {
			return errcontract.New(errcontract.CodeInvalidArguments, fmt.Sprintf("retail store-day fact import batch %s not found", strings.TrimSpace(*importBatchID)))
		}
		return fmt.Errorf("resolve retail store-day fact import batch %s: %w", strings.TrimSpace(*importBatchID), err)
	}
	if batchLegalEntity == nil || *batchLegalEntity != storeLegalEntity {
		return errcontract.New(errcontract.CodeConflict, fmt.Sprintf("retail store-day fact import batch %s does not belong to store legal entity %s", strings.TrimSpace(*importBatchID), storeLegalEntity))
	}
	return nil
}

func (r *OperatingFactsRepository) getRetailStoreDayFactByBusinessKey(ctx context.Context, entity access.EntityFilter, fact *RetailStoreDayFact) (*RetailStoreDayFact, error) {
	query := "SELECT " + retailStoreDayFactColumns + ` FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id
		WHERE f.store_id=$1 AND f.business_date=$2::date AND f.version=$3 AND f.source_system=$4`
	args := []any{fact.StoreID, fact.BusinessDate, fact.Version, fact.SourceSystem}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendStoreScopePredicate(ctx, query, args, len(args)+1, "s")
	row := r.db.QueryRow(ctx, query, args...)
	item, scanErr := scanRetailStoreDayFact(row)
	if scanErr == pgx.ErrNoRows {
		return nil, nil
	}
	if scanErr != nil {
		return nil, fmt.Errorf("scan old retail store-day fact: %w", scanErr)
	}
	return item, nil
}

func (r *OperatingFactsRepository) listRetailStoreDayFactsByIDs(ctx context.Context, entity access.EntityFilter, factIDs []string) ([]*RetailStoreDayFact, error) {
	if len(factIDs) == 0 {
		return []*RetailStoreDayFact{}, nil
	}
	query := "SELECT " + retailStoreDayFactColumns + ` FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id
		WHERE f.id::text=ANY($1::text[])`
	args := []any{factIDs}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query, args, _ = appendStoreScopePredicate(ctx, query, args, len(args)+1, "s")
	query += ` ORDER BY f.business_date,s.code,f.version DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list idempotent retail store-day facts: %w", err)
	}
	defer rows.Close()
	result := make([]*RetailStoreDayFact, 0, len(factIDs))
	for rows.Next() {
		item, err := scanRetailStoreDayFact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan idempotent retail store-day fact: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate idempotent retail store-day facts: %w", err)
	}
	if len(result) != len(factIDs) {
		return nil, errcontract.New(errcontract.CodeDataUnavailable, "idempotent retail store-day result is outside the caller scope or no longer exists")
	}
	return result, nil
}

func retailStoreDayFactFilter(ctx context.Context, entity access.EntityFilter, dateFrom, dateTo string, storeIDs []string) (string, []any, int) {
	if storeIDs == nil {
		storeIDs = []string{}
	}
	fromWhere := ` FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id
		WHERE f.business_date BETWEEN $1::date AND $2::date
		AND (cardinality($3::text[])=0 OR f.store_id::text=ANY($3::text[]))`
	args := []any{dateFrom, dateTo, storeIDs}
	if clause, arg, err := entity.SQLClause("s.legal_entity_id", len(args)+1); err != nil {
		// The filter helper is only reachable with a constructed filter; a
		// zero value must never degrade into unfiltered access.
		return "", nil, 0
	} else if clause != "" {
		fromWhere += " AND " + clause
		args = append(args, arg)
	}
	fromWhere, args, nextArg := appendStoreScopePredicate(ctx, fromWhere, args, len(args)+1, "s")
	return fromWhere, args, nextArg
}

type retailStoreDayFactScanner interface {
	Scan(...any) error
}

func scanRetailStoreDayFact(scanner retailStoreDayFactScanner) (*RetailStoreDayFact, error) {
	item := &RetailStoreDayFact{}
	if err := scanner.Scan(
		&item.ID, &item.StoreID, &item.StoreCode, &item.StoreName, &item.Brand, &item.Region,
		&item.BusinessDate, &item.Currency, &item.Revenue, &item.GrossProfit, &item.Transactions,
		&item.Footfall, &item.AreaSqm, &item.LaborCost, &item.FixedRent, &item.VariableRent,
		&item.NonLeaseCost, &item.OtherControllableCost, &item.SourceSystem, &item.SourceRecordID,
		&item.ImportBatchID, &item.AsOfAt, &item.Version, &item.ReconciliationStatus,
		&item.MappingStatus, &item.DataQualityStatus, &item.DataClassification,
		&item.SimulationDatasetVersion, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return item, nil
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
