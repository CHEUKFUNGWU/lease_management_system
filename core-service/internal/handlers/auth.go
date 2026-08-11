package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

var assignableRoles = map[string]struct{}{
	"admin":    {},
	"editor":   {},
	"reviewer": {},
	"approver": {},
	"auditor":  {},
	"readonly": {},
}

type AuthHandler struct {
	cfg          *config.Config
	userRepo     authUserStore
	roleRepo     roleAssignmentStore
	refreshStore authRefreshStore
}

type authUserStore interface {
	List(ctx context.Context) ([]*repository.User, error)
	Create(ctx context.Context, username, email, password, role string, legalEntityID *string) (*repository.User, error)
	GetByUsername(ctx context.Context, username string) (*repository.User, error)
	CheckPassword(user *repository.User, password string) bool
}

type roleAssignmentStore interface {
	AssignRolesToUser(ctx context.Context, userID string, roleCodes []string, assignedBy string) error
	GetUserRoleCodes(ctx context.Context, userID string) ([]string, error)
}

type authRefreshStore interface {
	Create(context.Context, *repository.AuthRefreshSession) error
	Rotate(context.Context, string, string, *repository.AuthRefreshSession) error
	Revoke(context.Context, string, string) error
	RevokeAll(context.Context, string) error
	List(context.Context, string) ([]*repository.AuthRefreshSession, error)
	RevokeByID(context.Context, string, string) error
}

func NewAuthHandler(cfg *config.Config, userRepo authUserStore, roleRepo roleAssignmentStore) *AuthHandler {
	return &AuthHandler{cfg: cfg, userRepo: userRepo, roleRepo: roleRepo}
}

func (h *AuthHandler) WithRefreshTokenStore(store authRefreshStore) *AuthHandler {
	if h == nil {
		return h
	}
	clone := *h
	clone.refreshStore = store
	return &clone
}

type RegisterRequest struct {
	Username      string  `json:"username" binding:"required,min=3,max=50"`
	Email         string  `json:"email" binding:"required,email"`
	Password      string  `json:"password" binding:"required,min=6"`
	Role          string  `json:"role" binding:"required"`
	LegalEntityID *string `json:"legal_entity_id"`
}

type AuthResponse struct {
	Token            string    `json:"token"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	UserID           string    `json:"user_id"`
	Username         string    `json:"username"`
	Role             string    `json:"role"`
	Roles            []string  `json:"roles"`
	LegalEntityID    *string   `json:"legal_entity_id"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "public registration is disabled. please contact an administrator"})
}

// AdminCreateUser allows admin to create new users
type AdminCreateUserRequest struct {
	Username      string   `json:"username" binding:"required,min=3,max=50"`
	Email         string   `json:"email" binding:"required,email"`
	Password      string   `json:"password" binding:"required,min=6"`
	Role          string   `json:"role"`
	Roles         []string `json:"roles"`
	LegalEntityID *string  `json:"legal_entity_id"`
	IsActive      bool     `json:"is_active"`
}

func isAssignableRole(role string) bool {
	_, ok := assignableRoles[role]
	return ok
}

func requestedRoles(req AdminCreateUserRequest) []string {
	seen := map[string]struct{}{}
	roles := make([]string, 0, len(req.Roles)+1)
	for _, role := range req.Roles {
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	if req.Role != "" {
		if _, exists := seen[req.Role]; !exists {
			roles = append(roles, req.Role)
		}
	}
	return roles
}

func (h *AuthHandler) AdminListUsers(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	data := make([]gin.H, 0, len(users))
	for _, user := range users {
		roles, roleErr := h.roleRepo.GetUserRoleCodes(c.Request.Context(), user.ID)
		if roleErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user roles"})
			return
		}
		data = append(data, gin.H{
			"id": user.ID, "username": user.Username, "email": user.Email,
			"role": user.Role, "roles": roles, "legal_entity_id": user.LegalEntityID,
			"is_active": user.IsActive, "created_at": user.CreatedAt, "updated_at": user.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  data,
		"total": len(users),
	})
}

func (h *AuthHandler) AdminCreateUser(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	roles := requestedRoles(req)
	if len(roles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one role is required"})
		return
	}
	for _, role := range roles {
		if isAssignableRole(role) {
			continue
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	sort.Strings(roles)

	// Check if user exists
	existing, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	// Create user
	primaryRole := roles[0]
	user, err := h.userRepo.Create(c.Request.Context(), req.Username, req.Email, req.Password, primaryRole, req.LegalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}
	assignedBy, _ := c.Get("user_id")
	assignedByID, _ := assignedBy.(string)
	if h.roleRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "role assignment is unavailable"})
		return
	}
	if err := h.roleRepo.AssignRolesToUser(c.Request.Context(), user.ID, roles, assignedByID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign roles"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id":         user.ID,
		"username":        user.Username,
		"role":            user.Role,
		"roles":           roles,
		"legal_entity_id": user.LegalEntityID,
		"message":         "user created successfully",
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user is inactive"})
		return
	}

	if !h.userRepo.CheckPassword(user, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	roles, err := h.roleRepo.GetUserRoleCodes(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load role assignments"})
		return
	}

	response, err := h.issueTokens(c.Request.Context(), user, roles, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Refresh exchanges a valid refresh credential for a newly signed access
// token and a rotated refresh token. Roles and legal-entity ownership are
// loaded again from the authoritative user/role stores; claims from the
// refresh token are never used as permissions.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication configuration is unavailable"})
		return
	}
	claims, err := middleware.ParseRefreshToken(req.RefreshToken, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	user, err := h.userRepo.GetByUsername(c.Request.Context(), claims.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if user == nil || !user.IsActive || user.ID != claims.UserID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token is no longer valid"})
		return
	}
	roles, err := h.roleRepo.GetUserRoleCodes(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load role assignments"})
		return
	}
	response, session, err := h.buildTokens(user, roles, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	if h.refreshStore != nil {
		if err := h.refreshStore.Rotate(c.Request.Context(), claims.TokenID, hashRefreshToken(req.RefreshToken), session); err != nil {
			if errors.Is(err, repository.ErrRefreshTokenInvalid) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token is no longer valid"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist refresh token rotation"})
			return
		}
	}
	c.JSON(http.StatusOK, response)
}

// Logout revokes the supplied refresh session. The access token is short lived
// and is not used as a bearer credential for logout authorization.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h == nil || h.cfg == nil || h.refreshStore == nil {
		c.JSON(http.StatusOK, gin.H{"revoked": true})
		return
	}
	claims, err := middleware.ParseRefreshToken(req.RefreshToken, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"revoked": true})
		return
	}
	if err := h.refreshStore.Revoke(c.Request.Context(), claims.TokenID, hashRefreshToken(req.RefreshToken)); err != nil && !errors.Is(err, repository.ErrRefreshTokenInvalid) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	if h == nil || h.refreshStore == nil {
		c.JSON(http.StatusOK, gin.H{"revoked": true})
		return
	}
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	if userIDString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user is required"})
		return
	}
	if err := h.refreshStore.RevokeAll(c.Request.Context(), userIDString); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke refresh sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *AuthHandler) ListSessions(c *gin.Context) {
	if h == nil || h.refreshStore == nil {
		c.JSON(http.StatusOK, gin.H{"sessions": []any{}})
		return
	}
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	if userIDString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user is required"})
		return
	}
	sessions, err := h.refreshStore.List(c.Request.Context(), userIDString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list refresh sessions"})
		return
	}
	views := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		if session == nil {
			continue
		}
		active := session.RevokedAt == nil && session.ExpiresAt.After(time.Now().UTC())
		views = append(views, gin.H{
			"id": session.ID, "created_at": session.CreatedAt, "expires_at": session.ExpiresAt,
			"revoked_at": session.RevokedAt, "replaced_by": session.ReplacedBy,
			"ip_address": session.IPAddress, "user_agent": session.UserAgent, "active": active,
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": views})
}

func (h *AuthHandler) RevokeSession(c *gin.Context) {
	if h == nil || h.refreshStore == nil {
		c.JSON(http.StatusOK, gin.H{"revoked": true})
		return
	}
	userID, _ := c.Get("user_id")
	userIDString, _ := userID.(string)
	if userIDString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user is required"})
		return
	}
	if err := h.refreshStore.RevokeByID(c.Request.Context(), userIDString, c.Param("id")); err != nil {
		if errors.Is(err, repository.ErrRefreshTokenInvalid) {
			c.JSON(http.StatusNotFound, gin.H{"error": "refresh session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke refresh session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *AuthHandler) issueTokens(ctx context.Context, user *repository.User, roles []string, sessionMetadata ...string) (AuthResponse, error) {
	response, session, err := h.buildTokens(user, roles, sessionMetadata...)
	if err != nil {
		return AuthResponse{}, err
	}
	if h.refreshStore != nil {
		if err := h.refreshStore.Create(ctx, session); err != nil {
			return AuthResponse{}, err
		}
	}
	return response, nil
}

func (h *AuthHandler) buildTokens(user *repository.User, roles []string, sessionMetadata ...string) (AuthResponse, *repository.AuthRefreshSession, error) {
	if h == nil || h.cfg == nil || user == nil {
		return AuthResponse{}, nil, errors.New("authentication configuration is unavailable")
	}
	var legalEntityID string
	if user.LegalEntityID != nil {
		legalEntityID = *user.LegalEntityID
	}
	accessTTL := middleware.DefaultAccessTokenTTL
	if h.cfg.AccessTokenTTLSeconds > 0 {
		accessTTL = time.Duration(h.cfg.AccessTokenTTLSeconds) * time.Second
	}
	refreshTTL := middleware.DefaultRefreshTokenTTL
	if h.cfg.RefreshTokenTTLSeconds > 0 {
		refreshTTL = time.Duration(h.cfg.RefreshTokenTTLSeconds) * time.Second
	}
	token, err := middleware.GenerateTokenWithRolesTTL(user.ID, user.Username, user.Role, roles, legalEntityID, h.cfg.JWTSecret, accessTTL)
	if err != nil {
		return AuthResponse{}, nil, err
	}
	refreshToken, refreshExpiresAt, err := middleware.GenerateRefreshToken(user.ID, user.Username, h.cfg.JWTSecret, refreshTTL)
	if err != nil {
		return AuthResponse{}, nil, err
	}
	refreshClaims, err := middleware.ParseRefreshToken(refreshToken, h.cfg.JWTSecret)
	if err != nil {
		return AuthResponse{}, nil, err
	}
	now := time.Now().UTC()
	response := AuthResponse{
		Token: token, RefreshToken: refreshToken, UserID: user.ID, Username: user.Username,
		Role: user.Role, Roles: roles, LegalEntityID: user.LegalEntityID,
		ExpiresAt: now.Add(accessTTL), RefreshExpiresAt: refreshExpiresAt,
	}
	var ipAddress, userAgent string
	if len(sessionMetadata) > 0 {
		ipAddress = sessionMetadata[0]
	}
	if len(sessionMetadata) > 1 {
		userAgent = sessionMetadata[1]
	}
	return response, &repository.AuthRefreshSession{
		UserID: user.ID, TokenID: refreshClaims.TokenID, TokenHash: hashRefreshToken(refreshToken), ExpiresAt: refreshExpiresAt,
		IPAddress: ipAddress, UserAgent: userAgent,
	}, nil
}

func hashRefreshToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
