package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	DefaultAccessTokenTTL  = 24 * time.Hour
	DefaultRefreshTokenTTL = 30 * 24 * time.Hour
)

func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if tokenType, exists := claims["token_type"].(string); exists && tokenType != "access" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "access token required"})
				c.Abort()
				return
			}
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
			c.Set("role", claims["role"])
			c.Set("roles", claims["roles"])
			c.Set("legal_entity_id", claims["legal_entity_id"])
		}

		c.Next()
	}
}

func GenerateToken(userID, username, role, legalEntityID, jwtSecret string) (string, error) {
	return GenerateTokenWithRoles(userID, username, role, []string{role}, legalEntityID, jwtSecret)
}

func GenerateTokenWithRoles(userID, username, role string, roles []string, legalEntityID, jwtSecret string) (string, error) {
	return GenerateTokenWithRolesTTL(userID, username, role, roles, legalEntityID, jwtSecret, DefaultAccessTokenTTL)
}

func GenerateTokenWithRolesTTL(userID, username, role string, roles []string, legalEntityID, jwtSecret string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = DefaultAccessTokenTTL
	}
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":         userID,
		"username":        username,
		"role":            role,
		"roles":           roles,
		"legal_entity_id": legalEntityID,
		"token_type":      "access",
		"exp":             now.Add(ttl).Unix(),
		"iat":             now.Unix(),
	})

	return token.SignedString([]byte(jwtSecret))
}

// GenerateRefreshToken creates a signed, narrowly scoped refresh credential.
// It carries identity only so the refresh handler can reload authoritative
// user roles from the database; permissions and tenant scope are never trusted
// from this token.
func GenerateRefreshToken(userID, username, jwtSecret string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = DefaultRefreshTokenTTL
	}
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"username":   username,
		"token_type": "refresh",
		"jti":        uuid.NewString(),
		"exp":        expiresAt.Unix(),
		"iat":        now.Unix(),
	})
	signed, err := token.SignedString([]byte(jwtSecret))
	return signed, expiresAt, err
}

type RefreshClaims struct {
	UserID   string
	Username string
	TokenID  string
}

func ParseRefreshToken(raw, jwtSecret string) (RefreshClaims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.TrimSpace(jwtSecret) == "" {
		return RefreshClaims{}, errors.New("refresh token and signing secret are required")
	}
	token, err := jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || token == nil || !token.Valid {
		return RefreshClaims{}, errors.New("invalid refresh token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return RefreshClaims{}, errors.New("invalid refresh token claims")
	}
	tokenType, _ := claims["token_type"].(string)
	if tokenType != "refresh" {
		return RefreshClaims{}, errors.New("token is not a refresh token")
	}
	username, _ := claims["username"].(string)
	userID, _ := claims["user_id"].(string)
	tokenID, _ := claims["jti"].(string)
	if strings.TrimSpace(username) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(tokenID) == "" {
		return RefreshClaims{}, errors.New("refresh token identity is incomplete")
	}
	return RefreshClaims{UserID: userID, Username: username, TokenID: tokenID}, nil
}
