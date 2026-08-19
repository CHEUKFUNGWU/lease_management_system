package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
)

// TrialBalanceVersion is one content-identified GL trial balance (ADR-0009).
type TrialBalanceVersion struct {
	ID                 string  `json:"id"`
	LegalEntityID      *string `json:"legal_entity_id,omitempty"`
	Name               string  `json:"name"`
	SourceSystem       string  `json:"source_system"`
	Period             string  `json:"period"`
	FunctionalCurrency string  `json:"functional_currency"`
	ContentSHA256      string  `json:"content_sha256"`
	TotalDebit         float64 `json:"total_debit"`
	TotalCredit        float64 `json:"total_credit"`
	CreatedBy          *string `json:"created_by,omitempty"`
	CreatedAt          string  `json:"created_at,omitempty"`
}

// TrialBalanceLine is one account row of a trial balance.
type TrialBalanceLine struct {
	ID                    string  `json:"id"`
	TrialBalanceVersionID string  `json:"trial_balance_version_id"`
	AccountCode           string  `json:"account_code"`
	AccountName           string  `json:"account_name,omitempty"`
	Debit                 float64 `json:"debit"`
	Credit                float64 `json:"credit"`
}

// CreateTrialBalanceVersion inserts a version idempotently on the content
// identity (entity, source, period, hash): a re-import replays the existing
// version instead of duplicating it.
func (r *OperatingFactsRepository) CreateTrialBalanceVersion(ctx context.Context, item *TrialBalanceVersion) (*TrialBalanceVersion, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO gl_trial_balance_versions (id,legal_entity_id,name,source_system,period,functional_currency,content_sha256,total_debit,total_credit,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (legal_entity_id, source_system, period, content_sha256) DO NOTHING
		RETURNING id, created_at`, item.ID, item.LegalEntityID, item.Name, item.SourceSystem, item.Period, item.FunctionalCurrency, item.ContentSHA256, item.TotalDebit, item.TotalCredit, item.CreatedBy).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrTrialBalanceVersionReplay
		}
		return nil, fmt.Errorf("create trial balance version: %w", err)
	}
	return item, nil
}

// ErrTrialBalanceVersionReplay signals the content identity already exists.
var ErrTrialBalanceVersionReplay = fmt.Errorf("trial balance content identity already exists")

// GetTrialBalanceVersionByContent resolves an existing version for replay.
func (r *OperatingFactsRepository) GetTrialBalanceVersionByContent(ctx context.Context, entity access.EntityFilter, sourceSystem, period, sha256 string) (*TrialBalanceVersion, error) {
	args := []any{sourceSystem, period, sha256}
	query := `SELECT id,legal_entity_id,name,source_system,period,functional_currency,content_sha256,total_debit,total_credit,created_by,created_at::text FROM gl_trial_balance_versions WHERE source_system=$1 AND period=$2 AND content_sha256=$3`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	item := &TrialBalanceVersion{}
	if err := r.db.QueryRow(ctx, query, args...).Scan(&item.ID, &item.LegalEntityID, &item.Name, &item.SourceSystem, &item.Period, &item.FunctionalCurrency, &item.ContentSHA256, &item.TotalDebit, &item.TotalCredit, &item.CreatedBy, &item.CreatedAt); err != nil {
		return nil, fmt.Errorf("resolve trial balance version: %w", err)
	}
	return item, nil
}

func (r *OperatingFactsRepository) CreateTrialBalanceLine(ctx context.Context, item *TrialBalanceLine) (*TrialBalanceLine, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO gl_trial_balance_lines (id,trial_balance_version_id,account_code,account_name,debit,credit) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (trial_balance_version_id, account_code) DO NOTHING`, item.ID, item.TrialBalanceVersionID, item.AccountCode, item.AccountName, item.Debit, item.Credit); err != nil {
		return nil, fmt.Errorf("create trial balance line: %w", err)
	}
	return item, nil
}

// ListTrialBalanceVersions returns the entity's trial balances, newest first.
func (r *OperatingFactsRepository) ListTrialBalanceVersions(ctx context.Context, entity access.EntityFilter, period string) ([]*TrialBalanceVersion, error) {
	args := []any{period}
	query := `SELECT id,legal_entity_id,name,source_system,period,functional_currency,content_sha256,total_debit,total_credit,created_by,created_at::text FROM gl_trial_balance_versions WHERE ($1='' OR period=$1)`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += ` ORDER BY period DESC, created_at DESC LIMIT 200`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trial balance versions: %w", err)
	}
	defer rows.Close()
	result := make([]*TrialBalanceVersion, 0)
	for rows.Next() {
		item := &TrialBalanceVersion{}
		if err := rows.Scan(&item.ID, &item.LegalEntityID, &item.Name, &item.SourceSystem, &item.Period, &item.FunctionalCurrency, &item.ContentSHA256, &item.TotalDebit, &item.TotalCredit, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// LatestTrialBalanceByPeriod returns, per accounting period, the newest TB
// version's lines plus that version's functional currency — the source the
// finmodel OpeningBalanceReader folds into standardized opening balances.
// Each version row already identifies the period; lines join by version id.
func (r *OperatingFactsRepository) LatestTrialBalanceByPeriod(ctx context.Context, legalEntityID string) (map[string][]TrialBalanceLine, string, error) {
	versions, err := r.ListTrialBalanceVersions(ctx, access.GlobalEntityFilter(), "")
	if err != nil {
		return nil, "", err
	}
	out := map[string][]TrialBalanceLine{}
	currency := ""
	picked := map[string]bool{}
	for _, version := range versions {
		if version.LegalEntityID == nil || *version.LegalEntityID != legalEntityID {
			continue
		}
		if picked[version.Period] {
			continue // 列表按 created_at DESC — 每个期间取最新版本
		}
		picked[version.Period] = true
		if currency == "" {
			currency = version.FunctionalCurrency
		}
		rows, err := r.db.Query(ctx, `SELECT id, trial_balance_version_id, account_code, account_name, debit, credit
			FROM gl_trial_balance_lines WHERE trial_balance_version_id=$1 ORDER BY account_code`, version.ID)
		if err != nil {
			return nil, "", fmt.Errorf("list trial balance lines: %w", err)
		}
		var lines []TrialBalanceLine
		for rows.Next() {
			line := TrialBalanceLine{}
			if err := rows.Scan(&line.ID, &line.TrialBalanceVersionID, &line.AccountCode, &line.AccountName, &line.Debit, &line.Credit); err != nil {
				rows.Close()
				return nil, "", fmt.Errorf("scan trial balance line: %w", err)
			}
			lines = append(lines, line)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, "", err
		}
		rows.Close()
		out[version.Period] = lines
	}
	return out, currency, nil
}

// DeleteTrialBalanceVersion compensates a failed import: removing the
// version cascades to its lines (P1-2).
func (r *OperatingFactsRepository) DeleteTrialBalanceVersion(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM gl_trial_balance_versions WHERE id=$1`, id); err != nil {
		return fmt.Errorf("delete trial balance version: %w", err)
	}
	return nil
}
