// 电商独立站模式仓储层：站点主数据、事实写入（请求级 + 业务级幂等、fact_version 重述）、
// ecomfact.FactReader 生产实现（Highest Fact Version 解析 + 跨法人行级过滤）、
// 收款对账 run 状态机与证据读取、GL 收入与固定费只读。
//
// 写入纪律（R-E1-3 / R-E2-2）：同一业务键重导以 max(fact_version)+1 追加，绝不 UPDATE 覆盖；
// 请求级幂等经 ecommerce_ingest_requests（scope × kind × key + payload sha），重放短路返回
// 原 record_ids；同键不同载荷 ⇒ 幂等冲突错误。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/ecomintake"
)

// Storefront 站点主数据行。
type Storefront struct {
	ID            string    `json:"id"`
	LegalEntityID string    `json:"legal_entity_id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Market        string    `json:"market"`
	Currency      string    `json:"currency"`
	Platform      string    `json:"platform"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EcommerceRepository 电商域全部读写。
type EcommerceRepository struct{ db DBTX }

// NewEcommerceRepository 构造。
func NewEcommerceRepository(db DBTX) *EcommerceRepository { return &EcommerceRepository{db: db} }

func scanStorefront(row pgx.Row) (*Storefront, error) {
	s := &Storefront{}
	err := row.Scan(&s.ID, &s.LegalEntityID, &s.Code, &s.Name, &s.Market, &s.Currency,
		&s.Platform, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

const storefrontColumns = `id, legal_entity_id::text, code, name, COALESCE(market,''), currency, platform, status, created_at, updated_at`

// ListStorefronts 法人行级过滤的站点列表（底线 1 从第一查询生效）。
func (r *EcommerceRepository) ListStorefronts(ctx context.Context, entity access.EntityFilter) ([]*Storefront, error) {
	query := "SELECT " + storefrontColumns + " FROM storefronts WHERE ($1 = '' OR status = 'active')"
	args := []any{"ok"}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY code"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list storefronts: %w", err)
	}
	defer rows.Close()
	out := []*Storefront{}
	for rows.Next() {
		s, err := scanStorefront(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetStorefront 单站点（含法人校验）。
func (r *EcommerceRepository) GetStorefront(ctx context.Context, entity access.EntityFilter, id string) (*Storefront, error) {
	query := "SELECT " + storefrontColumns + " FROM storefronts WHERE id::text = $1"
	args := []any{id}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	s, err := scanStorefront(r.db.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEcomNotFound
	}
	return s, err
}

// CreateStorefront 新建站点（管理员配置面）。
func (r *EcommerceRepository) CreateStorefront(ctx context.Context, s *Storefront) (*Storefront, error) {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Platform == "" {
		s.Platform = "shopify"
	}
	if s.Status == "" {
		s.Status = "active"
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO storefronts (id, legal_entity_id, code, name, market, currency, platform, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at, updated_at`,
		s.ID, s.LegalEntityID, s.Code, s.Name, s.Market, s.Currency, s.Platform, s.Status,
	).Scan(&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create storefront: %w", err)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// ecomfact.FactReader 生产实现：Highest Fact Version 解析在 SQL 里做
// （DISTINCT ON 业务键 ORDER BY fact_version DESC），跨法人行级过滤在这里兜底。

func appendEntity(clauses []string, args []any, entity access.EntityFilter, column string) ([]string, []any, error) {
	clause, arg, err := entity.SQLClause(column, len(args)+1)
	if err != nil {
		return nil, nil, err
	}
	if clause != "" {
		clauses = append(clauses, clause)
		args = append(args, arg)
	}
	return clauses, args, nil
}

func storefrontScope(entity access.EntityFilter, ids []string, w ecomfact.Window) (string, []any, error) {
	if ids == nil {
		ids = []string{} // nil slice 会被 pgx 序列化为 SQL NULL，cardinality(NULL)=NULL 让 WHERE 恒假
	}
	clauses := []string{"(cardinality($1::text[]) = 0 OR storefront_id::text = ANY($1::text[]))"}
	args := []any{ids}
	var err error
	clauses, args, err = appendEntity(clauses, args, entity, "legal_entity_id::text")
	if err != nil {
		return "", nil, err
	}
	clauses = append(clauses, fmt.Sprintf("business_date BETWEEN $%d::date AND $%d::date", len(args)+1, len(args)+2))
	args = append(args, w.From.Format(time.DateOnly), w.To.Format(time.DateOnly))
	return strings.Join(clauses, " AND "), args, nil
}


// StorefrontDays 实现 ecomfact.FactReader。多来源并存：DISTINCT ON（业务键×source_system）
// 取最高版本后由消费方按度量求和。
func (r *EcommerceRepository) StorefrontDays(ctx context.Context, f ecomfact.StorefrontFilter, w ecomfact.Window) ([]ecomfact.StorefrontDayFact, error) {
	where, args, err := storefrontScope(f.Entity, f.StorefrontIDs, w)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT DISTINCT ON (storefront_id::text, business_date, channel, sku, source_system)
			storefront_id::text, legal_entity_id::text, business_date, channel, sku, currency,
			gmv_amount, discount_amount, refund_amount, chargeback_loss_amount,
			order_count, new_customer_orders, landed_cost_amount, fulfillment_amount,
			payment_fee_amount, tax_collected_amount,
			source_system, import_batch_id::text, fact_version, as_of_at, data_classification,
			simulation_dataset_version, restated
		FROM storefront_day_facts
		WHERE ` + where + `
		ORDER BY storefront_id::text, business_date, channel, sku, source_system, fact_version DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query storefront day facts: %w", err)
	}
	defer rows.Close()
	out := []ecomfact.StorefrontDayFact{}
	for rows.Next() {
		fact, err := scanStorefrontDay(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *fact)
	}
	return out, rows.Err()
}

type ecomRowScanner interface{ Scan(...any) error }

func scanStorefrontDay(s ecomRowScanner) (*ecomfact.StorefrontDayFact, error) {
	f := &ecomfact.StorefrontDayFact{}
	env := &f.SourceEnvelope
	var simVersion *string
	if err := s.Scan(&f.StorefrontID, &f.LegalEntityID, &f.BusinessDate, &f.Channel, &f.SKU, &f.Currency,
		&f.GMVAmount, &f.DiscountAmount, &f.RefundAmount, &f.ChargebackLoss,
		&f.OrderCount, &f.NewCustomerOrders, &f.LandedCostAmount, &f.FulfillmentAmount,
		&f.PaymentFeeAmount, &f.TaxCollectedAmount,
		&env.SourceSystem, &env.ImportBatchID, &env.FactVersion, &env.AsOfAt, &env.DataClassification,
		&simVersion, &env.Restated); err != nil {
		return nil, err
	}
	env.SimulationDatasetVersion = simVersion
	return f, nil
}

// CampaignDays 实现 ecomfact.FactReader（basis 过滤 booked|paid）。
func (r *EcommerceRepository) CampaignDays(ctx context.Context, f ecomfact.StorefrontFilter, w ecomfact.Window, basis ecomfact.AdBasis) ([]ecomfact.CampaignDayFact, error) {
	if !basis.Valid() {
		return nil, fmt.Errorf("campaign days: 非法广告口径 %q（仅 booked|paid）", basis)
	}
	where, args, err := storefrontScope(f.Entity, f.StorefrontIDs, w)
	if err != nil {
		return nil, err
	}
	where = where + " AND basis = $" + strconv.Itoa(len(args)+1)
	args = append(args, string(basis))
	query := `
		SELECT DISTINCT ON (storefront_id::text, campaign_id, business_date, source_system)
			storefront_id::text, legal_entity_id::text, campaign_id, COALESCE(campaign_name,''),
			business_date, basis, COALESCE(media_owner,''), spend_amount, impressions, clicks, conversions,
			COALESCE(invoice_no,''), currency,
			source_system, import_batch_id::text, fact_version, as_of_at, data_classification,
			simulation_dataset_version, restated
		FROM campaign_day_facts
		WHERE ` + where + `
		ORDER BY storefront_id::text, campaign_id, business_date, source_system, fact_version DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query campaign day facts: %w", err)
	}
	defer rows.Close()
	out := []ecomfact.CampaignDayFact{}
	for rows.Next() {
		fact := &ecomfact.CampaignDayFact{}
		env := &fact.SourceEnvelope
		var simVersion *string
		var campaignName, mediaOwner, invoiceNo string
		if err := rows.Scan(&fact.StorefrontID, &fact.LegalEntityID, &fact.CampaignID, &campaignName,
			&fact.BusinessDate, &fact.Basis, &mediaOwner, &fact.SpendAmount, &fact.Impressions, &fact.Clicks, &fact.Conversions,
			&invoiceNo, &fact.Currency,
			&env.SourceSystem, &env.ImportBatchID, &env.FactVersion, &env.AsOfAt, &env.DataClassification,
			&simVersion, &env.Restated); err != nil {
			return nil, err
		}
		if campaignName != "" {
			cn := campaignName
			fact.CampaignName = &cn
		}
		if mediaOwner != "" {
			mo := mediaOwner
			fact.MediaOwner = &mo
		}
		if invoiceNo != "" {
			in := invoiceNo
			fact.InvoiceNo = &in
		}
		env.SimulationDatasetVersion = simVersion
		out = append(out, *fact)
	}
	return out, rows.Err()
}

// OrderLines 实现 ecomfact.FactReader：订单行证据路径，必须携带证据引用。
func (r *EcommerceRepository) OrderLines(ctx context.Context, ref ecomfact.EvidenceRef) ([]ecomfact.OrderLine, error) {
	if strings.TrimSpace(ref.StorefrontID) == "" || strings.TrimSpace(ref.PlatformOrderNo) == "" {
		return nil, fmt.Errorf("order lines: 证据引用不完整")
	}
	query := `
		SELECT DISTINCT ON (platform_order_no, line_no, source_system)
			platform_order_no, line_no, business_date, channel, sku, quantity,
			gross_amount, discount_amount, refund_amount, tax_amount, currency,
			COALESCE(payout_id,''), evidence,
			source_system, import_batch_id::text, fact_version, as_of_at, data_classification,
			simulation_dataset_version
		FROM order_line_evidence
		WHERE storefront_id::text = $1 AND platform_order_no = $2
		ORDER BY platform_order_no, line_no, source_system, fact_version DESC`
	rows, err := r.db.Query(ctx, query, ref.StorefrontID, ref.PlatformOrderNo)
	if err != nil {
		return nil, fmt.Errorf("query order lines: %w", err)
	}
	defer rows.Close()
	out := []ecomfact.OrderLine{}
	for rows.Next() {
		line := &ecomfact.OrderLine{}
		env := &line.Envelope
		var payoutID string
		var evidenceBytes []byte
		if err := rows.Scan(&line.PlatformOrderNo, &line.LineNo, &line.BusinessDate, &line.Channel, &line.SKU,
			&line.Quantity, &line.GrossAmount, &line.DiscountAmount, &line.RefundAmount, &line.TaxAmount,
			&line.Currency, &payoutID, &evidenceBytes,
			&env.SourceSystem, &env.ImportBatchID, &env.FactVersion, &env.AsOfAt, &env.DataClassification,
			&env.SimulationDatasetVersion); err != nil {
			return nil, err
		}
		if payoutID != "" {
			p := payoutID
			line.PayoutID = &p
		}
		if len(evidenceBytes) > 0 {
			_ = json.Unmarshal(evidenceBytes, &line.Evidence)
		}
		out = append(out, *line)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// GL 收入与固定费只读（sitepnl 的两个端口）

// GLRevenueRow sitepnl.GLRevenue 的仓储投影。
type GLRevenueRow struct {
	Amount        *float64  `json:"amount"`
	Currency      string    `json:"currency"`
	SourceSystem  string    `json:"source_system"`
	ImportBatchID string    `json:"import_batch_id"`
	FactVersion   int       `json:"fact_version"`
	AsOfAt        time.Time `json:"as_of_at"`
}

// LatestGLRevenue 取期间内最高版本的 GL 收入（会计口径唯一来源）。
func (r *EcommerceRepository) LatestGLRevenue(ctx context.Context, entity access.EntityFilter, storefrontID, period string) (*GLRevenueRow, error) {
	query := `
		SELECT revenue_amount, currency, source_system, import_batch_id::text, fact_version, as_of_at
		FROM storefront_gl_revenues
		WHERE storefront_id::text = $1 AND period = $2`
	args := []any{storefrontID, period}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY fact_version DESC LIMIT 1"
	row := &GLRevenueRow{}
	err := r.db.QueryRow(ctx, query, args...).Scan(&row.Amount, &row.Currency, &row.SourceSystem, &row.ImportBatchID, &row.FactVersion, &row.AsOfAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 未导入 ≠ 错误：调用方降级为 gl_unavailable Gap
	}
	if err != nil {
		return nil, fmt.Errorf("latest gl revenue: %w", err)
	}
	return row, nil
}

// LatestFixedCost 取期间内最高版本的分摊固定费。
func (r *EcommerceRepository) LatestFixedCost(ctx context.Context, entity access.EntityFilter, storefrontID, period string) (*FixedCostRow, error) {
	query := `
		SELECT fixed_cost_amount, currency, source_system
		FROM storefront_fixed_costs
		WHERE storefront_id::text = $1 AND period = $2`
	args := []any{storefrontID, period}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY fact_version DESC LIMIT 1"
	row := &FixedCostRow{}
	err := r.db.QueryRow(ctx, query, args...).Scan(&row.Amount, &row.Currency, &row.SourceSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest fixed cost: %w", err)
	}
	return row, nil
}

// FixedCostRow 分摊固定费读侧投影。
type FixedCostRow struct {
	Amount       *float64 `json:"amount"`
	Currency     string   `json:"currency"`
	SourceSystem string   `json:"source_system"`
}

// ---------------------------------------------------------------------------
// ecomintake.Sink 生产实现

// BeginBatch 建 operating_fact_batches 行（幂等键带 ecom: 前缀避免与零售批次键空间冲突）；
// 已有同键完成态批次 ⇒ replay=true。
func (r *EcommerceRepository) BeginBatch(_ context.Context, spec ecomintake.ImportSpec, totalRows int) (*ecomintake.BatchInfo, bool, error) {
	if _, err := access.EntityFilterFor(spec.LegalEntityID); err != nil {
		return nil, false, err
	}
	key := "ecom:" + spec.IdempotencyKey
	existing, err := r.findBatchByIdempotencyKey(context.Background(), key)
	if err != nil {
		return nil, false, err
	}
	if existing != nil && (existing.Status == "completed" || existing.Status == "failed") &&
		existing.AcceptedRows+existing.RejectedRows > 0 {
		return &ecomintake.BatchInfo{ID: existing.ID, Status: existing.Status,
			AcceptedRows: existing.AcceptedRows, RejectedRows: existing.RejectedRows}, true, nil
	}
	batch := &OperatingFactBatch{
		ID:                   uuid.NewString(),
		LegalEntityID:        &spec.LegalEntityID,
		SourceSystem:         spec.Envelope.SourceSystem,
		SourceFile:           spec.Filename,
		AsOfAt:               spec.Envelope.AsOfAt,
		Status:               "processing",
		TotalRows:            totalRows,
		ReconciliationStatus: "unreconciled",
		ErrorSummary:         json.RawMessage("[]"),
		CreatedBy:            spec.UserID,
		IdempotencyKey:       key,
		FactVersion:          time.Now().UTC().Format(time.RFC3339),
	}
	created, err := NewOperatingFactsRepository(r.db).CreateBatch(context.Background(), batch)
	if err != nil {
		return nil, false, fmt.Errorf("create ecom ingest batch: %w", err)
	}
	return &ecomintake.BatchInfo{ID: created.ID, Status: created.Status}, false, nil
}

func (r *EcommerceRepository) findBatchByIdempotencyKey(ctx context.Context, key string) (*OperatingFactBatch, error) {
	if strings.TrimSpace(key) == "" {
		return nil, nil
	}
	row := r.db.QueryRow(ctx, `
		SELECT id,status,total_rows,accepted_rows,rejected_rows FROM operating_fact_batches
		WHERE idempotency_key = $1 ORDER BY created_at DESC LIMIT 1`, key)
	batch := &OperatingFactBatch{}
	err := row.Scan(&batch.ID, &batch.Status, &batch.TotalRows, &batch.AcceptedRows, &batch.RejectedRows)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find ecom batch by idempotency key: %w", err)
	}
	return batch, nil
}

// FinalizeBatch 终态回写。
func (r *EcommerceRepository) FinalizeBatch(ctx context.Context, batchID string, accepted, rejected int, status, errorsJSON string) error {
	reconciliation := "unreconciled"
	if rejected > 0 {
		reconciliation = "warning"
	}
	_, err := NewOperatingFactsRepository(r.db).FinalizeBatch(ctx, batchID, accepted, rejected, status, reconciliation, json.RawMessage(errorsJSON))
	if err != nil {
		return fmt.Errorf("finalize ecom ingest batch: %w", err)
	}
	return nil
}

// CommitChunk 单块落库：一个事务里完成请求级幂等登记 + 每行业务键版本解析 + 插入。
// settlement 来源同时派生准备金事件；ad_invoice 来源的发票头走 RegisterInvoiceHeaders。
func (r *EcommerceRepository) CommitChunk(
	ctx context.Context,
	spec ecomintake.ImportSpec,
	table string,
	rows []ecomintake.ParsedRow,
	chunkKey, payloadSHA string,
) (*ecomintake.CommitResult, error) {
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("ecommerce chunk commit requires a PostgreSQL pool")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ecommerce chunk commit: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	txRepo := &EcommerceRepository{db: tx}

	scopeKey := "global"
	if spec.LegalEntityID != "" {
		scopeKey = spec.LegalEntityID
	}
	requestKind := "ingest:" + table
	var recordIDs []string
	if strings.TrimSpace(payloadSHA) != "" {
		var requestID string
		insertErr := tx.QueryRow(ctx, `
			INSERT INTO ecommerce_ingest_requests (scope_key,legal_entity_id,request_kind,idempotency_key,payload_sha256,created_by)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (scope_key,request_kind,idempotency_key) DO NOTHING
			RETURNING id`, scopeKey, nullableText(spec.LegalEntityID), requestKind, chunkKey, payloadSHA, nullableText(strDeref(spec.UserID))).Scan(&requestID)
		if insertErr == pgx.ErrNoRows {
			var storedHash string
			var rawIDs []byte
			if err := tx.QueryRow(ctx, `
				SELECT payload_sha256,record_ids FROM ecommerce_ingest_requests
				WHERE scope_key=$1 AND request_kind=$2 AND idempotency_key=$3`,
				scopeKey, requestKind, chunkKey).Scan(&storedHash, &rawIDs); err != nil {
				return nil, fmt.Errorf("load ecommerce idempotency record: %w", err)
			}
			if storedHash != payloadSHA {
				return nil, fmt.Errorf("%w: scope=%s key=%s", ErrEcomIdempotencyConflict, scopeKey, chunkKey)
			}
			if err := json.Unmarshal(rawIDs, &recordIDs); err != nil {
				return nil, fmt.Errorf("decode ecommerce idempotency record ids: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit ecommerce idempotent replay: %w", err)
			}
			committed = true
			return &ecomintake.CommitResult{SavedIDs: recordIDs, Replayed: true}, nil
		}
		if insertErr != nil {
			return nil, fmt.Errorf("persist ecommerce idempotency record: %w", insertErr)
		}
	}

	for _, row := range rows {
		id, err := txRepo.insertParsedRow(ctx, spec, table, row)
		if err != nil {
			return nil, err
		}
		recordIDs = append(recordIDs, id)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ecommerce_ingest_requests SET record_ids = $3::jsonb
		WHERE scope_key=$1 AND request_kind=$2 AND idempotency_key=$4`,
		scopeKey, requestKind, mustJSON(recordIDs), chunkKey); err != nil {
		return nil, fmt.Errorf("persist ecommerce idempotency record ids: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ecommerce chunk: %w", err)
	}
	committed = true
	return &ecomintake.CommitResult{SavedIDs: recordIDs}, nil
}

// RegisterInvoiceHeaders 发票头登记：ON CONFLICT DO NOTHING——发票号只登记一次，
// 同发票重复导入（或跨批次重复）不会产生第二条 ad_invoices 记录（业务级幂等 R-E1-3）。
func (r *EcommerceRepository) RegisterInvoiceHeaders(ctx context.Context, spec ecomintake.ImportSpec, invoices []ecomintake.InvoiceHeader) error {
	for _, inv := range invoices {
		_, err := r.db.Exec(ctx, `
			INSERT INTO ad_invoices (id, legal_entity_id, storefront_id, invoice_no, agent_name, media_owner,
				period_start, period_end, invoice_date, currency, gross_amount, rebate_amount, payable_amount,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			SELECT $1,$2,$3,$4,$5,$6,
				$7::date, $8::date, $9::date, $10, $11, $12, $13,
				$14, b.id, 1, $15, $16, NULLIF($17,'')
			FROM operating_fact_batches b WHERE b.idempotency_key = $18
			ON CONFLICT (invoice_no, source_system, fact_version) DO NOTHING`,
			uuid.NewString(), spec.LegalEntityID, spec.StorefrontID, inv.InvoiceNo,
			nullableText(strPtrOrEmpty(inv.AgentName)), nullableText(strPtrOrEmpty(inv.MediaOwner)),
			strPtrOrEmpty(inv.PeriodStart), strPtrOrEmpty(inv.PeriodEnd), strPtrOrEmpty(inv.InvoiceDate),
			inv.Currency, inv.GrossAmount, inv.RebateAmount, inv.PayableAmount,
			spec.Envelope.SourceSystem, spec.Envelope.AsOfAt, spec.Envelope.DataClassification,
			spec.Envelope.SimulationDatasetVersion, "ecom:"+spec.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("register ad invoice %s: %w", inv.InvoiceNo, err)
		}
	}
	return nil
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// insertParsedRow 按目标表分发插入；fact_version = 该业务键当前最大版本 + 1；
// 版本 > 1 ⇒ restated=true（重述标记，R-E2-2）。settlement 行派生准备金事件。
// execOne 执行 INSERT...SELECT 并强制恰好 1 行——JOIN 空转（0 行）是写入 bug，
// 静默吞掉会让报告 accepted=1 而库里 0 行。
func execOne(ctx context.Context, db DBTX, sql string, args ...any) error {
	tag, err := db.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("INSERT affected %d rows, want 1（JOIN 空转或重复键）", tag.RowsAffected())
	}
	return nil
}

func (r *EcommerceRepository) insertParsedRow(ctx context.Context, spec ecomintake.ImportSpec, table string, row ecomintake.ParsedRow) (string, error) {
	v := row.Values
	getStr := func(k string) any { return v[k] }
	id := uuid.NewString()
	classification := spec.Envelope.DataClassification
	simVersion := nullableText(spec.Envelope.SimulationDatasetVersion)

	switch table {
	case ecomintake.TableStorefrontDayFacts:
		date := getStr("business_date").(string)
		channel := strOr(getStr("channel"), "direct")
		sku := strOr(getStr("sku"), "")
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM storefront_day_facts
			WHERE storefront_id::text=$1 AND business_date=$2::date AND channel=$3 AND sku=$4 AND source_system=$5`,
			spec.StorefrontID, date, channel, sku, spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve storefront day version: %w", err)
		}
		version++
		if err := execOne(ctx, r.db, `
			INSERT INTO storefront_day_facts (id, legal_entity_id, storefront_id, business_date, channel, sku, currency,
				gmv_amount, discount_amount, refund_amount, chargeback_loss_amount, order_count, new_customer_orders,
				landed_cost_amount, fulfillment_amount, payment_fee_amount, tax_collected_amount,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version, restated, created_by)
			SELECT $1,s.legal_entity_id,s.id,$3::date,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,b.id,$18,$19,$20,$21,$22,$23
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$24
			WHERE s.id::text=$25 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, date, channel, sku, getStr("currency"),
			v["gmv_amount"], v["discount_amount"], v["refund_amount"], v["chargeback_loss_amount"],
			v["order_count"], v["new_customer_orders"], v["landed_cost_amount"], v["fulfillment_amount"],
			v["payment_fee_amount"], v["tax_collected_amount"],
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, simVersion, version > 1,
			nullableText(strDeref(spec.UserID)), "ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert storefront day fact: %w", err)
		}
	case ecomintake.TableCampaignDayFacts:
		basis := ecomintake.BasisBooked
		invoiceNo := any(nil)
		if spec.Source == ecomintake.SourceAdInvoice {
			basis = ecomintake.BasisPaid
			invoiceNo = getStr("invoice_no")
		}
		date := getStr("business_date").(string)
		campaign := strOr(getStr("campaign_id"), "all")
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM campaign_day_facts
			WHERE storefront_id::text=$1 AND campaign_id=$2 AND business_date=$3::date AND basis=$4 AND source_system=$5`,
			spec.StorefrontID, campaign, date, basis, spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve campaign day version: %w", err)
		}
		version++
		if _, err := r.db.Exec(ctx, `
			INSERT INTO campaign_day_facts (id, legal_entity_id, storefront_id, campaign_id, campaign_name, business_date, basis,
				media_owner, spend_amount, impressions, clicks, conversions, invoice_no, currency,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version, restated)
			SELECT $1,s.legal_entity_id,s.id,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,$12,$13,$14,b.id,$15,$16,$17,$18,$19
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$20
			WHERE s.id::text=$21 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, campaign, getStr("campaign_name"), date, basis,
			getStr("media_owner"), getOrZero(v, "spend_amount"), getIntPtr(v, "impressions"), getIntPtr(v, "clicks"),
			v["conversions"], invoiceNo, getStr("currency"),
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, simVersion, version > 1,
			"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert campaign day fact: %w", err)
		}
	case ecomintake.TablePayoutLines:
		payoutID := getStr("payout_id").(string)
		provider := getStr("provider").(string)
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM payout_lines
			WHERE storefront_id::text=$1 AND provider=$2 AND payout_id=$3 AND source_system=$4`,
			spec.StorefrontID, provider, payoutID, spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve payout version: %w", err)
		}
		version++
		if _, err := r.db.Exec(ctx, `
			INSERT INTO payout_lines (id, legal_entity_id, storefront_id, provider, payout_id, payout_date, currency,
				gross_amount, fee_amount, refund_amount, chargeback_amount, fx_amount, adjustment_amount,
				reserve_hold_amount, reserve_release_amount, net_amount,
				source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			SELECT $1,s.legal_entity_id,s.id,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,b.id,$17,$18,$19,$20
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$21
			WHERE s.id::text=$22 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, provider, payoutID, getStr("payout_date"), getStr("currency"),
			getOrZero(v, "gross_amount"), getOrZero(v, "fee_amount"), getOrZero(v, "refund_amount"),
			getOrZero(v, "chargeback_amount"), getOrZero(v, "fx_amount"), getOrZero(v, "adjustment_amount"),
			getOrZero(v, "reserve_hold_amount"), getOrZero(v, "reserve_release_amount"), getOrZero(v, "net_amount"),
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, simVersion,
			"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert payout line: %w", err)
		}
		// 准备金占用/释放事件派生（状态机载体）
		if hold := asFloat64(v["reserve_hold_amount"]); hold > 0 {
			if _, err := r.db.Exec(ctx, `
				INSERT INTO rolling_reserve_events (id, legal_entity_id, storefront_id, provider, event_type, event_date,
					currency, amount, payout_id, status, source_system, import_batch_id, fact_version, as_of_at,
					data_classification, simulation_dataset_version)
				SELECT $1,s.legal_entity_id,s.id,$3,'hold',$4::date,$5,$6,$7,'open',$8,b.id,1,$9,$10,NULLIF($11,'')
				FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$12 WHERE s.id::text=$13 AND s.legal_entity_id::text = $2::text`,
				uuid.NewString(), spec.LegalEntityID, provider, getStr("payout_date"), getStr("currency"),
				hold, payoutID, spec.Envelope.SourceSystem, spec.Envelope.AsOfAt, classification, spec.Envelope.SimulationDatasetVersion,
				"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
				return "", fmt.Errorf("insert reserve hold event: %w", err)
			}
		}
		if release := asFloat64(v["reserve_release_amount"]); release > 0 {
			var holdID *string
			_ = r.db.QueryRow(ctx, `
				SELECT id::text FROM rolling_reserve_events
				WHERE storefront_id::text=$1 AND provider=$2 AND payout_id=$3 AND event_type='hold' AND status='open'
				ORDER BY event_date LIMIT 1`, spec.StorefrontID, provider, payoutID).Scan(&holdID)
			if holdID != nil {
				if _, err := r.db.Exec(ctx, `
					INSERT INTO rolling_reserve_events (id, legal_entity_id, storefront_id, provider, event_type, event_date,
						currency, amount, payout_id, hold_event_id, released_at, status, source_system, import_batch_id,
						fact_version, as_of_at, data_classification, simulation_dataset_version)
					SELECT $1,s.legal_entity_id,s.id,$3,'release',$4::date,$5,$6,$7,$8::uuid,$4::date,'released',$9,b.id,1,$10,$11,NULLIF($12,'')
					FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$13 WHERE s.id::text=$14 AND s.legal_entity_id::text = $2::text`,
					uuid.NewString(), spec.LegalEntityID, provider, getStr("payout_date"), getStr("currency"),
					release, payoutID, *holdID, spec.Envelope.SourceSystem, spec.Envelope.AsOfAt, classification,
					spec.Envelope.SimulationDatasetVersion, "ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
					return "", fmt.Errorf("insert reserve release event: %w", err)
				}
				if _, err := r.db.Exec(ctx, `
					UPDATE rolling_reserve_events SET status='released', released_at=$3::date
					WHERE id::text=$1 AND status='open'`, *holdID, getStr("payout_date"), getStr("payout_date")); err != nil {
					return "", fmt.Errorf("close reserve hold: %w", err)
				}
			}
		}
	case ecomintake.TableBankLines:
		ref := getStr("bank_ref").(string)
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM bank_lines
			WHERE storefront_id::text=$1 AND bank_ref=$2 AND source_system=$3`,
			spec.StorefrontID, ref, spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve bank line version: %w", err)
		}
		version++
		direction := strOr(getStr("direction"), "in")
		if _, err := r.db.Exec(ctx, `
			INSERT INTO bank_lines (id, legal_entity_id, storefront_id, bank_ref, value_date, currency, amount,
				direction, counterparty, envelope, source_system, import_batch_id, fact_version, as_of_at,
				data_classification, simulation_dataset_version)
			SELECT $1,s.legal_entity_id,s.id,$3,$4::date,$5,$6,$7,$8,'{}'::jsonb,$9,b.id,$10,$11,$12,NULLIF($13,'')
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$14
			WHERE s.id::text=$15 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, ref, getStr("value_date"), getStr("currency"), getOrZero(v, "amount"),
			direction, getStr("counterparty"), spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt,
			classification, spec.Envelope.SimulationDatasetVersion, "ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert bank line: %w", err)
		}
	case ecomintake.TableOrderLineEvidence:
		orderNo := getStr("platform_order_no").(string)
		lineNo, ok := v["line_no"].(int)
		if !ok || lineNo < 1 {
			lineNo = 1
		}
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM order_line_evidence
			WHERE storefront_id::text=$1 AND platform_order_no=$2 AND line_no=$3 AND source_system=$4`,
			spec.StorefrontID, orderNo, lineNo, spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve order line version: %w", err)
		}
		version++
		if _, err := r.db.Exec(ctx, `
			INSERT INTO order_line_evidence (id, legal_entity_id, storefront_id, platform_order_no, line_no, business_date,
				channel, sku, quantity, gross_amount, discount_amount, refund_amount, tax_amount, currency, payout_id,
				evidence, source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			SELECT $1,s.legal_entity_id,s.id,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,$12,$13,$14,
				COALESCE($15::jsonb,'{}'::jsonb),$16,b.id,$17,$18,$19,NULLIF($20,'')
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$21
			WHERE s.id::text=$22 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, orderNo, lineNo, getStr("business_date"), strOr(getStr("channel"), "direct"),
			strOr(getStr("sku"), ""), v["quantity"], v["gross_amount"], v["discount_amount"], v["refund_amount"],
			v["tax_amount"], getStr("currency"), getStr("payout_id"), formatEvidence(v),
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, spec.Envelope.SimulationDatasetVersion,
			"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert order line evidence: %w", err)
		}
	case ecomintake.TableGLRevenues:
		period := getStr("period").(string)
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM storefront_gl_revenues
			WHERE storefront_id::text=$1 AND period=$2 AND currency=$3 AND source_system=$4`,
			spec.StorefrontID, period, getStr("currency"), spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve gl revenue version: %w", err)
		}
		version++
		if _, err := r.db.Exec(ctx, `
			INSERT INTO storefront_gl_revenues (id, legal_entity_id, storefront_id, period, currency, revenue_amount,
				gl_account, source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			SELECT $1,s.legal_entity_id,s.id,$3,$4,$5,$6,$7,b.id,$8,$9,$10,NULLIF($11,'')
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$12
			WHERE s.id::text=$13 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, period, getStr("currency"), v["revenue_amount"], getStr("gl_account"),
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, spec.Envelope.SimulationDatasetVersion,
			"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert gl revenue: %w", err)
		}
	case ecomintake.TableFixedCosts:
		period := getStr("period").(string)
		var version int
		if err := r.db.QueryRow(ctx, `
			SELECT COALESCE(MAX(fact_version),0) FROM storefront_fixed_costs
			WHERE storefront_id::text=$1 AND period=$2 AND currency=$3 AND source_system=$4`,
			spec.StorefrontID, period, getStr("currency"), spec.Envelope.SourceSystem).Scan(&version); err != nil {
			return "", fmt.Errorf("resolve fixed cost version: %w", err)
		}
		version++
		if _, err := r.db.Exec(ctx, `
			INSERT INTO storefront_fixed_costs (id, legal_entity_id, storefront_id, period, currency, fixed_cost_amount,
				memo, source_system, import_batch_id, fact_version, as_of_at, data_classification, simulation_dataset_version)
			SELECT $1,s.legal_entity_id,s.id,$3,$4,$5,$6,$7,b.id,$8,$9,$10,NULLIF($11,'')
			FROM storefronts s JOIN operating_fact_batches b ON b.idempotency_key=$12
			WHERE s.id::text=$13 AND s.legal_entity_id::text = $2::text`,
			id, spec.LegalEntityID, period, getStr("currency"), v["fixed_cost_amount"], getStr("memo"),
			spec.Envelope.SourceSystem, version, spec.Envelope.AsOfAt, classification, spec.Envelope.SimulationDatasetVersion,
			"ecom:"+spec.IdempotencyKey, spec.StorefrontID); err != nil {
			return "", fmt.Errorf("insert fixed cost: %w", err)
		}
	default:
		return "", fmt.Errorf("未知目标表 %q", table)
	}
	return id, nil
}

func formatEvidence(v map[string]any) string {
	ev := map[string]any{}
	for k, val := range v {
		switch k {
		case "platform_order_no", "line_no", "business_date", "channel", "sku", "quantity",
			"gross_amount", "discount_amount", "refund_amount", "tax_amount", "currency", "payout_id":
			continue
		default:
			ev[k] = val
		}
	}
	if len(ev) == 0 {
		return ""
	}
	return string(mustJSON(ev))
}

func strOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func getOrZero(v map[string]any, k string) any {
	if x, ok := v[k]; ok && x != nil {
		return x
	}
	return 0.0
}

func getIntPtr(v map[string]any, k string) any {
	if x, ok := v[k]; ok && x != nil {
		if n, isInt := x.(int); isInt {
			return n
		}
	}
	return nil
}

func asFloat64(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// 哨兵错误。
var (
	ErrEcomNotFound             = errors.New("ecommerce record not found")
	ErrEcomIdempotencyConflict  = errors.New("ecommerce idempotency conflict")
	ErrSettlementRunState       = errors.New("settlement run state transition not allowed")
)

// ---------------------------------------------------------------------------
// 对账证据读取与 run 状态机

// PayoutLineRow payout 明细读侧行。
type PayoutLineRow struct {
	Provider             string    `json:"provider"`
	PayoutID             string    `json:"payout_id"`
	PayoutDate           time.Time `json:"payout_date"`
	Currency             string    `json:"currency"`
	GrossAmount          float64   `json:"gross_amount"`
	FeeAmount            float64   `json:"fee_amount"`
	RefundAmount         float64   `json:"refund_amount"`
	ChargebackAmount     float64   `json:"chargeback_amount"`
	FXAmount             float64   `json:"fx_amount"`
	AdjustmentAmount     float64   `json:"adjustment_amount"`
	ReserveHoldAmount    float64   `json:"reserve_hold_amount"`
	ReserveReleaseAmount float64   `json:"reserve_release_amount"`
	NetAmount            float64   `json:"net_amount"`
	DataClassification   string    `json:"data_classification"`
}

// ListPayoutLines 窗口内的 payout 明细（最高版本）。
func (r *EcommerceRepository) ListPayoutLines(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]PayoutLineRow, error) {
	query := `
		SELECT DISTINCT ON (provider, payout_id)
			provider, payout_id, payout_date, currency, gross_amount, fee_amount, refund_amount,
			chargeback_amount, fx_amount, adjustment_amount, reserve_hold_amount, reserve_release_amount,
			net_amount, data_classification
		FROM payout_lines
		WHERE storefront_id::text = $1 AND payout_date BETWEEN $2::date AND $3::date`
	args := []any{storefrontID, from.Format(time.DateOnly), to.Format(time.DateOnly)}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY provider, payout_id, fact_version DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payout lines: %w", err)
	}
	defer rows.Close()
	out := []PayoutLineRow{}
	for rows.Next() {
		item := PayoutLineRow{}
		if err := rows.Scan(&item.Provider, &item.PayoutID, &item.PayoutDate, &item.Currency,
			&item.GrossAmount, &item.FeeAmount, &item.RefundAmount, &item.ChargebackAmount,
			&item.FXAmount, &item.AdjustmentAmount, &item.ReserveHoldAmount, &item.ReserveReleaseAmount,
			&item.NetAmount, &item.DataClassification); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ReceivableRow 按 payout 归集的订单应收（来自订单行证据表——对账路径允许触它）。
type ReceivableRow struct {
	PayoutID string  `json:"payout_id"`
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// ListReceivablesByPayout 对账专用聚合：Σ(gross − discount − refund) per payout_id。
// 只触订单行证据表的对账路径（R-E2-1：分析端点不触它）。
func (r *EcommerceRepository) ListReceivablesByPayout(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]ReceivableRow, error) {
	query := `
		WITH latest AS (
			SELECT DISTINCT ON (platform_order_no, line_no, source_system)
				platform_order_no, line_no, source_system, payout_id, currency,
				gross_amount, discount_amount, refund_amount, tax_amount
			FROM order_line_evidence
			WHERE storefront_id::text = $1 AND business_date BETWEEN $2::date AND $3::date
			  AND payout_id IS NOT NULL`
	args := []any{storefrontID, from.Format(time.DateOnly), to.Format(time.DateOnly)}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += `
			ORDER BY platform_order_no, line_no, source_system, fact_version DESC
		)
		SELECT payout_id, currency,
			COALESCE(SUM(COALESCE(gross_amount,0) - COALESCE(discount_amount,0) - COALESCE(refund_amount,0)),0) AS amount
		FROM latest GROUP BY payout_id, currency ORDER BY payout_id`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list receivables by payout: %w", err)
	}
	defer rows.Close()
	out := []ReceivableRow{}
	for rows.Next() {
		item := ReceivableRow{}
		if err := rows.Scan(&item.PayoutID, &item.Currency, &item.Amount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// BankLineRow 银行到账读侧行。
type BankLineRow struct {
	BankRef  string    `json:"bank_ref"`
	ValueDate time.Time `json:"value_date"`
	Currency string    `json:"currency"`
	Amount   float64   `json:"amount"`
	DataClassification string `json:"data_classification"`
}

// ListBankLines 窗口内的银行到账（最高版本，方向=in）。
func (r *EcommerceRepository) ListBankLines(ctx context.Context, entity access.EntityFilter, storefrontID string, from, to time.Time) ([]BankLineRow, error) {
	query := `
		SELECT DISTINCT ON (bank_ref)
			bank_ref, value_date, currency, amount, data_classification
		FROM bank_lines
		WHERE storefront_id::text = $1 AND value_date BETWEEN $2::date AND $3::date AND direction='in'`
	args := []any{storefrontID, from.Format(time.DateOnly), to.Format(time.DateOnly)}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY bank_ref, fact_version DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list bank lines: %w", err)
	}
	defer rows.Close()
	out := []BankLineRow{}
	for rows.Next() {
		item := BankLineRow{}
		if err := rows.Scan(&item.BankRef, &item.ValueDate, &item.Currency, &item.Amount, &item.DataClassification); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ReserveEventRow 准备金台账读侧行。
type ReserveEventRow struct {
	ID          string    `json:"id"`
	Provider    string    `json:"provider"`
	EventType   string    `json:"event_type"`
	EventDate   time.Time `json:"event_date"`
	Currency    string    `json:"currency"`
	Amount      float64   `json:"amount"`
	PayoutID    *string   `json:"payout_id,omitempty"`
	HoldEventID *string   `json:"hold_event_id,omitempty"`
	Status      string    `json:"status"`
}

// ListReserveEvents 准备金事件（现金流预测输入，R-E4-2）。
func (r *EcommerceRepository) ListReserveEvents(ctx context.Context, entity access.EntityFilter, storefrontID string) ([]ReserveEventRow, error) {
	query := `
		SELECT DISTINCT ON (id::text) id::text, provider, event_type, event_date, currency, amount,
			payout_id, hold_event_id::text, status
		FROM rolling_reserve_events WHERE storefront_id::text = $1`
	args := []any{storefrontID}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY id::text, fact_version DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list reserve events: %w", err)
	}
	defer rows.Close()
	out := []ReserveEventRow{}
	for rows.Next() {
		item := ReserveEventRow{}
		if err := rows.Scan(&item.ID, &item.Provider, &item.EventType, &item.EventDate, &item.Currency,
			&item.Amount, &item.PayoutID, &item.HoldEventID, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SettlementRun 对账 run 行（签认状态机 Draft → Prepare → Pending → Approved/Rejected）。
type SettlementRun struct {
	ID                    string          `json:"id"`
	LegalEntityID         string          `json:"legal_entity_id"`
	StorefrontID          string          `json:"storefront_id"`
	Period                string          `json:"period"`
	Currency              string          `json:"currency"`
	Status                string          `json:"status"`
	PolicyVersion         string          `json:"policy_version"`
	GateVerdict           *string         `json:"gate_verdict,omitempty"`
	MatchedCount          int             `json:"matched_count"`
	DifferenceCount       int             `json:"difference_count"`
	TotalDifferenceAmount float64         `json:"total_difference_amount"`
	Results               json.RawMessage `json:"results"`
	Differences           json.RawMessage `json:"differences"`
	PreparedBy            *string         `json:"prepared_by,omitempty"`
	PreparedAt            *time.Time      `json:"prepared_at,omitempty"`
	SubmittedBy           *string         `json:"submitted_by,omitempty"`
	SubmittedAt           *time.Time      `json:"submitted_at,omitempty"`
	ApprovedBy            *string         `json:"approved_by,omitempty"`
	ApprovedAt            *time.Time      `json:"approved_at,omitempty"`
	RejectedBy            *string         `json:"rejected_by,omitempty"`
	RejectedAt            *time.Time      `json:"rejected_at,omitempty"`
	RejectionReason       *string         `json:"rejection_reason,omitempty"`
	CreatedBy             *string         `json:"created_by,omitempty"`
	IdempotencyKey        *string         `json:"idempotency_key,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

func scanSettlementRun(s ecomRowScanner) (*SettlementRun, error) {
	r := &SettlementRun{}
	err := s.Scan(&r.ID, &r.LegalEntityID, &r.StorefrontID, &r.Period, &r.Currency, &r.Status,
		&r.PolicyVersion, &r.GateVerdict, &r.MatchedCount, &r.DifferenceCount, &r.TotalDifferenceAmount,
		&r.Results, &r.Differences, &r.PreparedBy, &r.PreparedAt, &r.SubmittedBy, &r.SubmittedAt,
		&r.ApprovedBy, &r.ApprovedAt, &r.RejectedBy, &r.RejectedAt, &r.RejectionReason,
		&r.CreatedBy, &r.IdempotencyKey, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

const settlementRunColumns = `id, legal_entity_id::text, storefront_id::text, period, currency, status,
	policy_version, gate_verdict, matched_count, difference_count, total_difference_amount,
	results, differences, prepared_by::text, prepared_at, submitted_by::text, submitted_at,
	approved_by::text, approved_at, rejected_by::text, rejected_at, rejection_reason,
	created_by::text, idempotency_key, created_at, updated_at`

// CreateSettlementRun 新建对账 run（draft 态）；幂等键冲突时返回既有 run。
func (r *EcommerceRepository) CreateSettlementRun(ctx context.Context, run *SettlementRun) (*SettlementRun, bool, error) {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.PolicyVersion == "" {
		run.PolicyVersion = "settlement-match-v1"
	}
	run.Status = "draft"
	err := r.db.QueryRow(ctx, `
		INSERT INTO settlement_runs (id, legal_entity_id, storefront_id, period, currency, status, policy_version,
			gate_verdict, matched_count, difference_count, total_difference_amount,
			results, differences, created_by, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,'draft',$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''))
		ON CONFLICT (legal_entity_id, idempotency_key) DO UPDATE SET updated_at = NOW()
		RETURNING ` + settlementRunColumns,
		run.ID, run.LegalEntityID, run.StorefrontID, run.Period, run.Currency, run.PolicyVersion,
		run.GateVerdict, run.MatchedCount, run.DifferenceCount, run.TotalDifferenceAmount,
		run.Results, run.Differences, run.CreatedBy, strDeref(run.IdempotencyKey)).Scan(
		&run.ID, &run.LegalEntityID, &run.StorefrontID, &run.Period, &run.Currency, &run.Status,
		&run.PolicyVersion, &run.GateVerdict, &run.MatchedCount, &run.DifferenceCount, &run.TotalDifferenceAmount,
		&run.Results, &run.Differences, &run.PreparedBy, &run.PreparedAt, &run.SubmittedBy, &run.SubmittedAt,
		&run.ApprovedBy, &run.ApprovedAt, &run.RejectedBy, &run.RejectedAt, &run.RejectionReason,
		&run.CreatedBy, &run.IdempotencyKey, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		return nil, false, fmt.Errorf("create settlement run: %w", err)
	}
	return run, false, nil
}

// GetSettlementRun 单 run。
func (r *EcommerceRepository) GetSettlementRun(ctx context.Context, entity access.EntityFilter, id string) (*SettlementRun, error) {
	query := "SELECT " + settlementRunColumns + " FROM settlement_runs WHERE id::text = $1"
	args := []any{id}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	run, err := scanSettlementRun(r.db.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEcomNotFound
	}
	return run, err
}

// ListSettlementRuns 列表（按期间倒序）。
func (r *EcommerceRepository) ListSettlementRuns(ctx context.Context, entity access.EntityFilter, storefrontID, period string) ([]*SettlementRun, error) {
	query := "SELECT " + settlementRunColumns + " FROM settlement_runs WHERE ($1='' OR storefront_id::text=$1) AND ($2='' OR period=$2)"
	args := []any{storefrontID, period}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += " ORDER BY period DESC, created_at DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list settlement runs: %w", err)
	}
	defer rows.Close()
	out := []*SettlementRun{}
	for rows.Next() {
		run, err := scanSettlementRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// TransitionSettlementRun 状态机推进。合法迁移：
//
//	draft→prepared（prepare 动作，记 prepared_by）
//	prepared→pending（submit，记 submitted_by）
//	pending→approved / pending→rejected（审批，记 approved_by/rejected_by）
//
// 其余迁移一律拒绝（职责分离留痕，R-E4-4）。
func (r *EcommerceRepository) TransitionSettlementRun(ctx context.Context, entity access.EntityFilter, id, action, actorID, reason string) (*SettlementRun, error) {
	beginner, ok := r.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("settlement transition requires a PostgreSQL pool")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin settlement transition: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	current, err := scanSettlementRun(tx.QueryRow(ctx,
		"SELECT "+settlementRunColumns+" FROM settlement_runs WHERE id::text=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEcomNotFound
	}
	if err != nil {
		return nil, err
	}
	// 行级隔离再校验（FOR UPDATE 后仍要确认法人归属）
	if !entity.IsGlobal() {
		if eid, eidErr := entity.LegalEntityID(); eidErr == nil && current.LegalEntityID != eid {
			return nil, ErrEcomNotFound
		}
	}

	actor := nullableText(actorID)
	switch action {
	case "prepare":
		if current.Status != "draft" {
			return nil, fmt.Errorf("%w: draft→%s 不允许（当前 %s）", ErrSettlementRunState, action, current.Status)
		}
		_, err = tx.Exec(ctx, `UPDATE settlement_runs SET status='prepared', prepared_by=$2, prepared_at=NOW(), updated_at=NOW() WHERE id::text=$1`, id, actor)
	case "submit":
		if current.Status != "prepared" {
			return nil, fmt.Errorf("%w: prepared→pending 不允许（当前 %s）", ErrSettlementRunState, current.Status)
		}
		_, err = tx.Exec(ctx, `UPDATE settlement_runs SET status='pending', submitted_by=$2, submitted_at=NOW(), updated_at=NOW() WHERE id::text=$1`, id, actor)
	case "approve":
		if current.Status != "pending" {
			return nil, fmt.Errorf("%w: pending→approved 不允许（当前 %s）", ErrSettlementRunState, current.Status)
		}
		if current.GateVerdict != nil && *current.GateVerdict == "deny" {
			return nil, fmt.Errorf("%w: 口径门禁 deny 的期间不得进入 Approved（R-E4-3）", ErrSettlementRunState)
		}
		_, err = tx.Exec(ctx, `UPDATE settlement_runs SET status='approved', approved_by=$2, approved_at=NOW(), updated_at=NOW() WHERE id::text=$1`, id, actor)
	case "reject":
		if current.Status != "pending" {
			return nil, fmt.Errorf("%w: pending→rejected 不允许（当前 %s）", ErrSettlementRunState, current.Status)
		}
		_, err = tx.Exec(ctx, `UPDATE settlement_runs SET status='rejected', rejected_by=$2, rejected_at=NOW(), rejection_reason=$3, updated_at=NOW() WHERE id::text=$1`, id, actor, nullableText(reason))
	default:
		return nil, fmt.Errorf("未知状态动作 %q", action)
	}
	if err != nil {
		return nil, fmt.Errorf("apply settlement transition: %w", err)
	}
	updated, err := scanSettlementRun(tx.QueryRow(ctx,
		"SELECT "+settlementRunColumns+" FROM settlement_runs WHERE id::text=$1", id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit settlement transition: %w", err)
	}
	committed = true
	return updated, nil
}

// UpdateSettlementResults 回写匹配结果与门禁裁决（draft/prepared 态可重算）。
func (r *EcommerceRepository) UpdateSettlementResults(ctx context.Context, entity access.EntityFilter, id string, matchedCount, diffCount int, totalDiff float64, verdict string, results, differences json.RawMessage) error {
	query := `
		UPDATE settlement_runs SET matched_count=$3, difference_count=$4, total_difference_amount=$5,
		       gate_verdict=$6, results=$7, differences=$8, updated_at=NOW()
		WHERE id::text=$1 AND status IN ('draft','prepared')`
	args := []any{id, 0, matchedCount, diffCount, totalDiff, verdict, results, differences}
	if clause, arg, err := entity.SQLClause("legal_entity_id::text", len(args)+1); err != nil {
		return err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update settlement results: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEcomNotFound
	}
	return nil
}
