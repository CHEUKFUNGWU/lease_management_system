package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeAuthUserStore struct {
	created *repository.User
	lookup  *repository.User
}

func (s *fakeAuthUserStore) List(context.Context) ([]*repository.User, error) {
	return nil, nil
}

func (s *fakeAuthUserStore) Create(_ context.Context, username, email, _ string, role string, legalEntityID *string) (*repository.User, error) {
	s.created = &repository.User{
		ID:            "user-1",
		Username:      username,
		Email:         email,
		Role:          role,
		LegalEntityID: legalEntityID,
		IsActive:      true,
	}
	return s.created, nil
}

func (s *fakeAuthUserStore) GetByUsername(context.Context, string) (*repository.User, error) {
	return s.lookup, nil
}

func (s *fakeAuthUserStore) CheckPassword(*repository.User, string) bool {
	return true
}

type fakeRoleAssignmentStore struct {
	userID           string
	roleCodes        []string
	assignedByID     string
	roleCodesForUser []string
}

func (s *fakeRoleAssignmentStore) GetUserRoleCodes(context.Context, string) ([]string, error) {
	return append([]string(nil), s.roleCodesForUser...), nil
}

func (s *fakeRoleAssignmentStore) AssignRolesToUser(_ context.Context, userID string, roleCodes []string, assignedBy string) error {
	s.userID = userID
	s.roleCodes = append([]string(nil), roleCodes...)
	s.assignedByID = assignedBy
	return nil
}

func TestLoginReturnsEveryAuthoritativeRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{lookup: &repository.User{
		ID:       "user-1",
		Username: "finance_user",
		Role:     "editor",
		IsActive: true,
	}}
	roles := &fakeRoleAssignmentStore{roleCodesForUser: []string{"editor", "reviewer"}}
	handler := NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, users, roles)

	router := gin.New()
	router.POST("/login", handler.Login)
	body := []byte(`{"username":"finance_user","password":"password123"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Roles []string `json:"roles"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !reflect.DeepEqual(response.Roles, []string{"editor", "reviewer"}) {
		t.Fatalf("expected authoritative roles, got %#v", response.Roles)
	}
}

func TestInactiveUserCannotLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{lookup: &repository.User{
		ID:       "user-1",
		Username: "disabled_user",
		Role:     "readonly",
		IsActive: false,
	}}
	roles := &fakeRoleAssignmentStore{roleCodesForUser: []string{"readonly"}}
	handler := NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, users, roles)

	router := gin.New()
	router.POST("/login", handler.Login)
	body := []byte(`{"username":"disabled_user","password":"password123"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected inactive user login to return %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAdminCreateUserAssignsEveryRequestedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{}
	roles := &fakeRoleAssignmentStore{}
	handler := NewAuthHandler(&config.Config{}, users, roles)

	router := gin.New()
	router.POST("/users", func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("user_id", "admin-1")
		handler.AdminCreateUser(c)
	})

	body := []byte(`{"username":"finance_user","email":"finance@example.com","password":"password123","roles":["editor","reviewer"],"is_active":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if roles.userID != "user-1" {
		t.Fatalf("expected roles assigned to user-1, got %q", roles.userID)
	}
	if !reflect.DeepEqual(roles.roleCodes, []string{"editor", "reviewer"}) {
		t.Fatalf("expected both role assignments, got %#v", roles.roleCodes)
	}
	if roles.assignedByID != "admin-1" {
		t.Fatalf("expected admin-1 as assigner, got %q", roles.assignedByID)
	}
}
