package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SavedView is one presentation-config row of fin_saved_views (S3-5). The
// config is validated by finmodel/view.Lint before it ever reaches here —
// a saved view never carries data or a permission.
type SavedView struct {
	ID            string          `json:"id"`
	LegalEntityID string          `json:"legal_entity_id"`
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	Config        json.RawMessage `json:"config"`
	IsShared      bool            `json:"is_shared"`
	IsDefault     bool            `json:"is_default"`
	CreatedBy     *string         `json:"created_by,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// ErrSavedViewNotFound covers both a missing row and a row the caller may
// not see — the two are indistinguishable to a caller by design (no oracle
// for other tenants' rows).
var ErrSavedViewNotFound = errors.New("saved view not found or not visible to caller")

const savedViewCols = "id, legal_entity_id, kind, name, config, is_shared, is_default, created_by, created_at, updated_at"

func scanSavedView(s rowScanner) (*SavedView, error) {
	var v SavedView
	err := s.Scan(&v.ID, &v.LegalEntityID, &v.Kind, &v.Name, &v.Config,
		&v.IsShared, &v.IsDefault, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	return &v, err
}

// CreateSavedView inserts the view and, when isDefault is set, makes it the
// caller's personal default for that surface. Clear-then-insert runs in one
// statement so the (created_by, kind) default uniqueness index never sees
// two defaults; the COUNT(*) reference gives the insert a genuine data
// dependency on cleared — PostgreSQL does not order dependency-free
// data-modifying CTEs.
func (r *FinModelRepository) CreateSavedView(ctx context.Context, v *SavedView) error {
	err := r.db.QueryRow(ctx, `WITH cleared AS (
			UPDATE fin_saved_views SET is_default=false, updated_at=NOW()
			WHERE created_by=$8 AND kind=$3 AND is_default
			RETURNING id
		), ins AS (
			INSERT INTO fin_saved_views
				(id, legal_entity_id, kind, name, config, is_shared, is_default, created_by)
			SELECT $1,$2,$3,$4,$5,$6,$7,$8
			WHERE (SELECT COUNT(*) FROM cleared) IS NOT NULL
			RETURNING id
		)
		SELECT id FROM ins`,
		v.ID, v.LegalEntityID, v.Kind, v.Name, string(v.Config), v.IsShared, v.IsDefault, v.CreatedBy).Scan(&v.ID)
	return err
}

// GetSavedViewForUser reads one view the caller may see: their own, or one
// shared inside their legal entity. A nil legalEntityID is the global-admin
// wildcard — non-admin users are rejected upstream by RequireTenant, and a
// nil host variable keeps the uuid comparison correctly typed.
func (r *FinModelRepository) GetSavedViewForUser(ctx context.Context, id string, legalEntityID *string, userID string) (*SavedView, error) {
	row, err := scanSavedView(r.db.QueryRow(ctx, `SELECT `+savedViewCols+` FROM fin_saved_views
		WHERE id=$1 AND (legal_entity_id=$2 OR $2 IS NULL) AND (created_by=$3 OR is_shared)
		ORDER BY created_at DESC LIMIT 1`, id, legalEntityID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSavedViewNotFound
	}
	return row, err
}

// UpdateSavedView changes name/config of a view the caller owns. Sharing is
// not settable here — the dedicated share action requires its own
// permission, so a write-permission bug cannot widen visibility.
func (r *FinModelRepository) UpdateSavedView(ctx context.Context, id, userID string, name *string, config json.RawMessage) error {
	tag, err := r.db.Exec(ctx, `UPDATE fin_saved_views SET name=COALESCE($3::varchar, name),
		config=COALESCE($4::jsonb, config), updated_at=NOW()
		WHERE id=$1 AND created_by=$2`, id, userID, name, nullableJSONText(config))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSavedViewNotFound
	}
	return nil
}

// DeleteSavedView removes a view the caller owns.
func (r *FinModelRepository) DeleteSavedView(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM fin_saved_views WHERE id=$1 AND created_by=$2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSavedViewNotFound
	}
	return nil
}

// ListSavedViews returns the caller's own views plus views shared inside
// their legal entity, newest first. Nil legalEntityID is the global-admin
// wildcard only (see GetSavedViewForUser).
func (r *FinModelRepository) ListSavedViews(ctx context.Context, legalEntityID *string, kind, userID string) ([]*SavedView, error) {
	rows, err := r.db.Query(ctx, `SELECT `+savedViewCols+` FROM fin_saved_views
		WHERE (legal_entity_id=$1 OR $1 IS NULL) AND kind=$2 AND (created_by=$3 OR is_shared)
		ORDER BY is_default DESC, created_at DESC`, legalEntityID, kind, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SavedView
	for rows.Next() {
		row, err := scanSavedView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SetDefaultSavedView makes a caller-owned view their personal default for
// its surface. One statement, so the partial unique index on
// (created_by, kind) WHERE is_default is the transaction boundary; the
// EXISTS reference forces chosen to run after cleared (see CreateSavedView).
func (r *FinModelRepository) SetDefaultSavedView(ctx context.Context, id, kind, userID string) error {
	err := r.db.QueryRow(ctx, `WITH cleared AS (
			UPDATE fin_saved_views SET is_default=false, updated_at=NOW()
			WHERE created_by=$2 AND kind=$3 AND is_default
			RETURNING id
		), chosen AS (
			UPDATE fin_saved_views SET is_default=true, updated_at=NOW()
			WHERE id=$1 AND created_by=$2 AND (SELECT COUNT(*) FROM cleared) IS NOT NULL
			RETURNING id
		)
		SELECT id FROM chosen`, id, userID, kind).Scan(new(string))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSavedViewNotFound
	}
	return err
}

// ShareSavedView toggles entity-wide visibility of a caller-owned view.
func (r *FinModelRepository) ShareSavedView(ctx context.Context, id, userID string, shared bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE fin_saved_views SET is_shared=$3, updated_at=NOW()
		WHERE id=$1 AND created_by=$2`, id, userID, shared)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSavedViewNotFound
	}
	return nil
}

// nullableJSONText is the pgx-friendly jsonb wrapper: text encoding coerces
// into jsonb on the server without pgx guessing bytea for []byte, and nil
// scans as SQL NULL so an omitted field keeps its stored value.
func nullableJSONText(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	return string(raw)
}
