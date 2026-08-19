package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/finmodel/view"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

// SavedViewHandler serves the S3-5 saved views: named presentation configs
// (period range, version lines, basis mode, grain, row show/hide, filters).
// Views never carry data — Lint enforces that fail-closed — and every
// mutation is owner-guarded in the repository, so a shared view widens
// visibility of a config, never of rows.
type SavedViewHandler struct {
	repo *repository.FinModelRepository
}

// NewSavedViewHandler builds the handler.
func NewSavedViewHandler(repo *repository.FinModelRepository) *SavedViewHandler {
	return &SavedViewHandler{repo: repo}
}

type savedViewRequest struct {
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	Config        json.RawMessage `json:"config,omitempty"`
	IsShared      bool            `json:"is_shared"`
	IsDefault     bool            `json:"is_default"`
	LegalEntityID string          `json:"legal_entity_id,omitempty"`
}

// normalize validates the request shape and returns the shared
// entity/kind/name/config tuple used by create and update.
func (r savedViewRequest) normalize(existingKind, existingName string) (kind, legalEntityID, name string, cfg json.RawMessage, err error) {
	switch {
	case existingKind != "":
		kind = existingKind
	default:
		kind = strings.TrimSpace(r.Kind)
		if !view.ValidKind(kind) {
			return "", "", "", nil, errors.New("kind is required and must be store_pnl, financial_model or group_view")
		}
	}
	name = r.Name
	if name == "" {
		name = existingName
	}
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 200 {
		return "", "", "", nil, errors.New("name must be 1-200 characters")
	}
	cfg = r.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}
	normalized, err := view.Lint(view.Kind(kind), cfg)
	if err != nil {
		return "", "", "", nil, err
	}
	stored, err := json.Marshal(normalized)
	if err != nil {
		return "", "", "", nil, err
	}
	legalEntityID = r.LegalEntityID
	return kind, legalEntityID, name, stored, nil
}

// entityFor answers which legal entity a create anchors to: the caller's
// tenant, or — for a global admin with no tenant — the explicit body field.
// An empty answer is a 400: a saved view must belong to a tenant (bottom
// line 1).
func entityFor(c *gin.Context, bodyEntity string) (string, bool) {
	tenant := middleware.GetTenantID(c)
	if tenant == "" {
		tenant = strings.TrimSpace(bodyEntity)
	}
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal entity scope is required for saved views"})
		return "", false
	}
	return tenant, true
}

func (h *SavedViewHandler) List(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	if !view.ValidKind(kind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind query is required and must be store_pnl, financial_model or group_view"})
		return
	}
	userID, ok := userID(c)
	if !ok {
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	views, err := h.repo.ListSavedViews(c.Request.Context(), tenantOrNil(c), kind, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"views": views})
}

func (h *SavedViewHandler) Create(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var req savedViewRequest
	if err := decodeStrictJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	kind, _, name, cfg, err := req.normalize("", "")
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	entity, present := entityFor(c, req.LegalEntityID)
	if !present {
		return
	}
	// Sharing is a separate, narrower grant (fin_views:share): a write
	// permission alone must not be able to widen visibility.
	if req.IsShared && !callerHasPermission(c, "fin_views", "share") {
		c.JSON(http.StatusForbidden, gin.H{"error": "sharing a view requires the fin_views:share permission"})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	created := &repository.SavedView{
		ID: uuid.NewString(), LegalEntityID: entity, Kind: kind, Name: name,
		Config: cfg, IsShared: req.IsShared, IsDefault: req.IsDefault, CreatedBy: &userID,
	}
	if err := h.repo.CreateSavedView(c.Request.Context(), created); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"view": created})
}

func (h *SavedViewHandler) Update(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req savedViewRequest
	if err := decodeStrictJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	existing, err := h.repo.GetSavedViewForUser(c.Request.Context(), id, tenantOrNil(c), userID)
	if err != nil {
		writeSavedViewError(c, err)
		return
	}
	if existing.CreatedBy == nil || *existing.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the owner may update a saved view"})
		return
	}
	_, _, name, cfg, err := req.normalize(existing.Kind, existing.Name)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if !req.hasMutation() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	if err := h.repo.UpdateSavedView(c.Request.Context(), id, userID, optString(req.Name != "" && strings.TrimSpace(req.Name) != "", name), cfg); err != nil {
		writeSavedViewError(c, err)
		return
	}
	updated, err := h.repo.GetSavedViewForUser(c.Request.Context(), id, tenantOrNil(c), userID)
	if err != nil {
		writeSavedViewError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"view": updated})
}

func (s savedViewRequest) hasMutation() bool {
	return s.Name != "" || s.Config != nil
}

func (h *SavedViewHandler) Delete(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	if err := h.repo.DeleteSavedView(c.Request.Context(), c.Param("id"), userID); err != nil {
		writeSavedViewError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// SetDefault marks one of the caller's own views as their personal default.
// A shared view cannot become someone else's default — the default slot is
// owned by the creator — instead the recipient saves their own copy.
func (h *SavedViewHandler) SetDefault(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	existing, err := h.repo.GetSavedViewForUser(c.Request.Context(), id, tenantOrNil(c), userID)
	if err != nil {
		writeSavedViewError(c, err)
		return
	}
	if existing.CreatedBy == nil || *existing.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "a personal default must be one of your own views"})
		return
	}
	if err := h.repo.SetDefaultSavedView(c.Request.Context(), id, existing.Kind, userID); err != nil {
		writeSavedViewError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"view_id": id, "is_default": true})
}

// Share toggles entity-wide visibility of the owner's view. The route
// holds the fin_views:share permission itself.
func (h *SavedViewHandler) Share(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var req struct {
		Shared bool `json:"shared"`
	}
	if err := decodeStrictJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	if err := h.repo.ShareSavedView(c.Request.Context(), c.Param("id"), userID, req.Shared); err != nil {
		writeSavedViewError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"view_id": c.Param("id"), "is_shared": req.Shared})
}

func writeSavedViewError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrSavedViewNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func callerHasPermission(c *gin.Context, resource, action string) bool {
	permissions, _ := c.Get("permissions")
	perms, _ := permissions.([]string)
	return middleware.HasPermission(perms, resource, action)
}

// tenantOrNil mirrors the repository wildcard convention: an empty tenant
// is the global-admin case (RequireTenant guarantees nobody else reaches
// it) and is encoded as SQL NULL so the uuid comparison keeps its type.
func tenantOrNil(c *gin.Context) *string {
	if tenant := middleware.GetTenantID(c); tenant != "" {
		return &tenant
	}
	return nil
}

func optString(cond bool, value string) *string {
	if !cond {
		return nil
	}
	return &value
}
