package handlers

import (
	"context"
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
	cfg      *config.Config
	userRepo authUserStore
	roleRepo roleAssignmentStore
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

func NewAuthHandler(cfg *config.Config, userRepo authUserStore, roleRepo roleAssignmentStore) *AuthHandler {
	return &AuthHandler{cfg: cfg, userRepo: userRepo, roleRepo: roleRepo}
}

type RegisterRequest struct {
	Username      string  `json:"username" binding:"required,min=3,max=50"`
	Email         string  `json:"email" binding:"required,email"`
	Password      string  `json:"password" binding:"required,min=6"`
	Role          string  `json:"role" binding:"required"`
	LegalEntityID *string `json:"legal_entity_id"`
}

type AuthResponse struct {
	Token         string    `json:"token"`
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	Role          string    `json:"role"`
	Roles         []string  `json:"roles"`
	LegalEntityID *string   `json:"legal_entity_id"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
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
	// Check if current user is admin
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

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
	// Check if current user is admin
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

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

	var legalEntityID string
	if user.LegalEntityID != nil {
		legalEntityID = *user.LegalEntityID
	}
	roles, err := h.roleRepo.GetUserRoleCodes(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load role assignments"})
		return
	}

	token, err := middleware.GenerateTokenWithRoles(user.ID, user.Username, user.Role, roles, legalEntityID, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token:         token,
		UserID:        user.ID,
		Username:      user.Username,
		Role:          user.Role,
		Roles:         roles,
		LegalEntityID: user.LegalEntityID,
		ExpiresAt:     time.Now().Add(time.Hour * 24),
	})
}
