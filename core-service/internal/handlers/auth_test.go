package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

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

type fakeRefreshStore struct {
	sessions map[string]*repository.AuthRefreshSession
}

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{sessions: make(map[string]*repository.AuthRefreshSession)}
}

func (s *fakeRefreshStore) Create(_ context.Context, session *repository.AuthRefreshSession) error {
	if session.ID == "" {
		session.ID = session.TokenID
	}
	copy := *session
	s.sessions[session.TokenID] = &copy
	return nil
}

func (s *fakeRefreshStore) Rotate(_ context.Context, tokenID, tokenHash string, replacement *repository.AuthRefreshSession) error {
	current := s.sessions[tokenID]
	if current == nil || current.TokenHash != tokenHash || current.RevokedAt != nil || !current.ExpiresAt.After(time.Now()) {
		return repository.ErrRefreshTokenInvalid
	}
	now := time.Now()
	current.RevokedAt = &now
	current.ReplacedBy = &replacement.TokenID
	return s.Create(context.Background(), replacement)
}

func (s *fakeRefreshStore) Revoke(_ context.Context, tokenID, tokenHash string) error {
	current := s.sessions[tokenID]
	if current == nil || current.TokenHash != tokenHash {
		return repository.ErrRefreshTokenInvalid
	}
	now := time.Now()
	current.RevokedAt = &now
	return nil
}

func (s *fakeRefreshStore) RevokeAll(_ context.Context, userID string) error {
	now := time.Now()
	for _, session := range s.sessions {
		if session.UserID == userID {
			session.RevokedAt = &now
		}
	}
	return nil
}

func (s *fakeRefreshStore) List(_ context.Context, userID string) ([]*repository.AuthRefreshSession, error) {
	result := make([]*repository.AuthRefreshSession, 0)
	for _, session := range s.sessions {
		if session.UserID != userID {
			continue
		}
		copy := *session
		result = append(result, &copy)
	}
	return result, nil
}

func (s *fakeRefreshStore) RevokeByID(_ context.Context, userID, sessionID string) error {
	for _, session := range s.sessions {
		if session.ID != sessionID || session.UserID != userID {
			continue
		}
		now := time.Now()
		session.RevokedAt = &now
		return nil
	}
	return repository.ErrRefreshTokenInvalid
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

func TestLoginAndRefreshRotateAuthoritativeTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{lookup: &repository.User{
		ID: "user-1", Username: "finance_user", Role: "editor", IsActive: true,
	}}
	roles := &fakeRoleAssignmentStore{roleCodesForUser: []string{"editor", "reviewer"}}
	handler := NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, users, roles)
	router := gin.New()
	router.POST("/login", handler.Login)
	router.POST("/refresh", handler.Refresh)

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte(`{"username":"finance_user","password":"password123"}`)))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var loginResponse struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loginResponse); err != nil {
		t.Fatal(err)
	}
	if loginResponse.Token == "" || loginResponse.RefreshToken == "" || loginResponse.Token == loginResponse.RefreshToken {
		t.Fatalf("login tokens not issued separately: %+v", loginResponse)
	}

	refreshRecorder := httptest.NewRecorder()
	refreshRequest := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(`{"refresh_token":"`+loginResponse.RefreshToken+`"}`))
	refreshRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(refreshRecorder, refreshRequest)
	if refreshRecorder.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshRecorder.Code, refreshRecorder.Body.String())
	}
	var refreshResponse struct {
		Token        string   `json:"token"`
		RefreshToken string   `json:"refresh_token"`
		Roles        []string `json:"roles"`
	}
	if err := json.Unmarshal(refreshRecorder.Body.Bytes(), &refreshResponse); err != nil {
		t.Fatal(err)
	}
	if refreshResponse.Token == "" || refreshResponse.RefreshToken == "" || refreshResponse.RefreshToken == loginResponse.RefreshToken {
		t.Fatalf("refresh did not rotate credentials: %+v", refreshResponse)
	}
	if !reflect.DeepEqual(refreshResponse.Roles, []string{"editor", "reviewer"}) {
		t.Fatalf("refresh roles=%v", refreshResponse.Roles)
	}
}

func TestPersistentRefreshTokenIsSingleUseAndLogoutAllRevokesSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{lookup: &repository.User{
		ID: "user-1", Username: "finance_user", Role: "editor", IsActive: true,
	}}
	roles := &fakeRoleAssignmentStore{roleCodesForUser: []string{"editor"}}
	refreshStore := newFakeRefreshStore()
	handler := NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, users, roles).WithRefreshTokenStore(refreshStore)
	router := gin.New()
	router.POST("/login", handler.Login)
	router.POST("/refresh", handler.Refresh)
	router.POST("/logout-all", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.LogoutAll(c)
	})

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"finance_user","password":"password123"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("User-Agent", "lease-agent-test/1.0")
	loginRequest.Header.Set("X-Forwarded-For", "203.0.113.10")
	router.ServeHTTP(loginRecorder, loginRequest)
	var loginResponse AuthResponse
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loginResponse); err != nil {
		t.Fatal(err)
	}
	if len(refreshStore.sessions) != 1 {
		t.Fatalf("persistent sessions after login=%d", len(refreshStore.sessions))
	}
	for _, session := range refreshStore.sessions {
		if session.UserAgent != "lease-agent-test/1.0" || session.IPAddress == "" {
			t.Fatalf("session metadata was not captured: %+v", session)
		}
	}

	refreshRecorder := httptest.NewRecorder()
	refreshRequest := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"refresh_token":"`+loginResponse.RefreshToken+`"}`))
	refreshRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(refreshRecorder, refreshRequest)
	if refreshRecorder.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshRecorder.Code, refreshRecorder.Body.String())
	}

	replayRecorder := httptest.NewRecorder()
	replayRequest := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"refresh_token":"`+loginResponse.RefreshToken+`"}`))
	replayRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(replayRecorder, replayRequest)
	if replayRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh status=%d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}

	logoutAll := httptest.NewRecorder()
	router.ServeHTTP(logoutAll, httptest.NewRequest(http.MethodPost, "/logout-all", nil))
	if logoutAll.Code != http.StatusOK {
		t.Fatalf("logout-all status=%d body=%s", logoutAll.Code, logoutAll.Body.String())
	}
	for tokenID, session := range refreshStore.sessions {
		if session.RevokedAt == nil {
			t.Fatalf("session %s remained active after logout-all", tokenID)
		}
	}
}

func TestRefreshSessionEndpointsAreScopedToAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &fakeAuthUserStore{lookup: &repository.User{
		ID: "user-1", Username: "finance_user", Role: "editor", IsActive: true,
	}}
	roles := &fakeRoleAssignmentStore{roleCodesForUser: []string{"editor"}}
	refreshStore := newFakeRefreshStore()
	handler := NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, users, roles).WithRefreshTokenStore(refreshStore)
	router := gin.New()
	router.POST("/login", handler.Login)
	router.GET("/sessions", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.ListSessions(c)
	})
	router.DELETE("/sessions/:id", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.RevokeSession(c)
	})

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"finance_user","password":"password123"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}

	var loginResponse AuthResponse
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loginResponse); err != nil {
		t.Fatal(err)
	}
	sessionsRecorder := httptest.NewRecorder()
	router.ServeHTTP(sessionsRecorder, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if sessionsRecorder.Code != http.StatusOK {
		t.Fatalf("list sessions status=%d body=%s", sessionsRecorder.Code, sessionsRecorder.Body.String())
	}
	var sessionsResponse struct {
		Sessions []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(sessionsRecorder.Body.Bytes(), &sessionsResponse); err != nil {
		t.Fatal(err)
	}
	if len(sessionsResponse.Sessions) != 1 || sessionsResponse.Sessions[0].ID == "" || !sessionsResponse.Sessions[0].Active {
		t.Fatalf("unexpected sessions response: %+v", sessionsResponse)
	}

	revokeRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokeRecorder, httptest.NewRequest(http.MethodDelete, "/sessions/"+sessionsResponse.Sessions[0].ID, nil))
	if revokeRecorder.Code != http.StatusOK {
		t.Fatalf("revoke session status=%d body=%s", revokeRecorder.Code, revokeRecorder.Body.String())
	}

	sessionsRecorder = httptest.NewRecorder()
	router.ServeHTTP(sessionsRecorder, httptest.NewRequest(http.MethodGet, "/sessions", nil))
	if sessionsRecorder.Code != http.StatusOK {
		t.Fatalf("list revoked sessions status=%d body=%s", sessionsRecorder.Code, sessionsRecorder.Body.String())
	}
	if err := json.Unmarshal(sessionsRecorder.Body.Bytes(), &sessionsResponse); err != nil {
		t.Fatal(err)
	}
	if len(sessionsResponse.Sessions) != 1 || sessionsResponse.Sessions[0].Active {
		t.Fatalf("revoked session remained active: %+v", sessionsResponse)
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

	body := []byte(`{"username":"finance_user","email":"finance@example.com","password":"password123","roles":["editor","reviewer"],"legal_entity_id":"11111111-1111-1111-1111-111111111111","is_active":true}`)
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

func adminCreateUserRouter(t *testing.T, users *fakeAuthUserStore, roles *fakeRoleAssignmentStore, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(&config.Config{}, users, roles)
	router := gin.New()
	router.POST("/users", func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("user_id", "admin-1")
		handler.AdminCreateUser(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

// T5: creating a non-admin user without legal_entity_id must be refused with
// 400; otherwise the user would silently become a cross-legal-entity reader.
func TestAdminCreateUserRequiresLegalEntityForNonAdmin(t *testing.T) {
	users := &fakeAuthUserStore{}
	roles := &fakeRoleAssignmentStore{}
	body := `{"username":"no_le_user","email":"no_le@example.com","password":"password123","roles":["editor"],"is_active":true}`

	recorder := adminCreateUserRouter(t, users, roles, body)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing legal_entity_id to return %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if users.created != nil {
		t.Fatalf("expected no user to be created, got %#v", users.created)
	}
}

// T6: a non-admin user with a malformed legal_entity_id must be refused with
// 400 rather than storing a value that never matches any row.
func TestAdminCreateUserRejectsInvalidLegalEntityUUID(t *testing.T) {
	users := &fakeAuthUserStore{}
	roles := &fakeRoleAssignmentStore{}
	body := `{"username":"bad_le_user","email":"bad_le@example.com","password":"password123","roles":["editor"],"legal_entity_id":"not-a-uuid","is_active":true}`

	recorder := adminCreateUserRouter(t, users, roles, body)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid UUID to return %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if users.created != nil {
		t.Fatalf("expected no user to be created, got %#v", users.created)
	}
}

// T7: the admin role is a legitimate cross-legal-entity role, so an admin
// user without legal_entity_id must still be created.
func TestAdminCreateUserAllowsMissingLegalEntityForAdmin(t *testing.T) {
	users := &fakeAuthUserStore{}
	roles := &fakeRoleAssignmentStore{}
	body := `{"username":"admin_2","email":"admin2@example.com","password":"password123","roles":["admin"],"is_active":true}`

	recorder := adminCreateUserRouter(t, users, roles, body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected admin user creation to succeed, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
