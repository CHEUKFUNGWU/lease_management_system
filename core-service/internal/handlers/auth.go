package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	
	"github.com/ifrs16/core-service/internal/config"
	"github.com/ifrs16/core-service/internal/middleware"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=admin reviewer approver user"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// In-memory user store for MVP (replace with database in production)
var users = make(map[string]*User)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

func Register(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		if _, exists := users[req.Username]; exists {
			c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
			return
		}
		
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		
		user := &User{
			ID:       uuid.New().String(),
			Username: req.Username,
			Email:    req.Email,
			Password: string(hashedPassword),
			Role:     req.Role,
		}
		users[req.Username] = user
		
		token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}
		
		c.JSON(http.StatusCreated, AuthResponse{
			Token:     token,
			Username:  user.Username,
			Role:      user.Role,
			ExpiresAt: time.Now().Add(time.Hour * 24),
		})
	}
}

func Login(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		user, exists := users[req.Username]
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		
		token, err := middleware.GenerateToken(user.ID, user.Username, user.Role, cfg.JWTSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}
		
		c.JSON(http.StatusOK, AuthResponse{
			Token:     token,
			Username:  user.Username,
			Role:      user.Role,
			ExpiresAt: time.Now().Add(time.Hour * 24),
		})
	}
}
