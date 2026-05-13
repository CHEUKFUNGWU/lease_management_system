package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/config"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type AuthHandler struct {
	cfg      *config.Config
	userRepo *repository.UserRepository
}

func NewAuthHandler(cfg *config.Config, userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{cfg: cfg, userRepo: userRepo}
}

type RegisterRequest struct {
	Username        string  `json:"username" binding:"required,min=3,max=50"`
	Email           string  `json:"email" binding:"required,email"`
	Password        string  `json:"password" binding:"required,min=6"`
	Role            string  `json:"role" binding:"required,oneof=admin reviewer approver user"`
	LegalEntityID   *string `json:"legal_entity_id"`
}

type AuthResponse struct {
	Token         string    `json:"token"`
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	Role          string    `json:"role"`
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
	Username      string  `json:"username" binding:"required,min=3,max=50"`
	Email         string  `json:"email" binding:"required,email"`
	Password      string  `json:"password" binding:"required,min=6"`
	Role          string  `json:"role" binding:"required,oneof=admin reviewer approver user"`
	LegalEntityID *string `json:"legal_entity_id"`
	IsActive      bool    `json:"is_active"`
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

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
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
	user, err := h.userRepo.Create(c.Request.Context(), req.Username, req.Email, req.Password, req.Role, req.LegalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id":         user.ID,
		"username":        user.Username,
		"role":            user.Role,
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
	
	if !h.userRepo.CheckPassword(user, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	
	var legalEntityID string
	if user.LegalEntityID != nil {
		legalEntityID = *user.LegalEntityID
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, legalEntityID, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token:         token,
		UserID:        user.ID,
		Username:      user.Username,
		Role:          user.Role,
		LegalEntityID: user.LegalEntityID,
		ExpiresAt:     time.Now().Add(time.Hour * 24),
	})
}
