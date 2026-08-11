package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestRefreshTokenCarriesOnlyRefreshIdentityAndRejectsAccessToken(t *testing.T) {
	refreshToken, _, err := GenerateRefreshToken("user-1", "finance_user", "secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseRefreshToken(refreshToken, "secret")
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "finance_user" {
		t.Fatalf("claims=%+v", claims)
	}

	accessToken, err := GenerateTokenWithRoles("user-1", "finance_user", "editor", []string{"editor"}, "le-001", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRefreshToken(accessToken, "secret"); err == nil {
		t.Fatal("access token must not be accepted as refresh token")
	}
}

func TestRefreshTokenRejectsWrongSecret(t *testing.T) {
	refreshToken, _, err := GenerateRefreshToken("user-1", "finance_user", "secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRefreshToken(refreshToken, "wrong-secret"); err == nil {
		t.Fatal("refresh token signed by another secret must be rejected")
	}
}

func TestJWTAuthRejectsRefreshTokenOnProtectedRoute(t *testing.T) {
	refresh, _, err := GenerateRefreshToken("user-1", "finance_user", "secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuth("secret"))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+refresh)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestJWTAuthAcceptsLegacyTokenWithoutTokenType(t *testing.T) {
	now := time.Now().UTC()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user-1", "username": "legacy", "role": "editor", "roles": []string{"editor"},
		"legal_entity_id": "le-001", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(),
	}).SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuth("secret"))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+signed)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("legacy status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestJWTAuthRejectsExpiredAccessToken(t *testing.T) {
	now := time.Now().UTC()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user-1", "username": "expired", "role": "editor", "roles": []string{"editor"},
		"legal_entity_id": "le-001", "token_type": "access", "exp": now.Add(-time.Minute).Unix(), "iat": now.Add(-2 * time.Minute).Unix(),
	}).SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuth("secret"))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+signed)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired token status=%d body=%s", response.Code, response.Body.String())
	}
}
