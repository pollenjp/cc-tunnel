package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// newServerWithCredService creates a Server that has credService enabled (auth required).
func newServerWithCredService() (*Server, string) {
	s := &Server{
		repo:        &mockRepoCheckCtx{conv: makeConv("00000001-0000-0000-0000-000000000001")},
		session:     newAppSession(),
		credService: &mockCredService{credJSON: []byte(`{}`)},
	}
	token := "valid-token-conv-auth"
	s.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})
	return s, token
}

// TestListConversations_NoBearer_Returns401 verifies that ListConversations returns 401
// when credService is configured but no Authorization header is sent.
func TestListConversations_NoBearer_Returns401(t *testing.T) {
	s, _ := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)

	s.ListConversations(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestListConversations_ValidBearer_Returns200 verifies that ListConversations returns 200
// when a valid Bearer token is provided.
func TestListConversations_ValidBearer_Returns200(t *testing.T) {
	s, token := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	s.ListConversations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateConversation_NoBearer_Returns401 verifies that CreateConversation returns 401
// when credService is configured but no Authorization header is sent.
func TestCreateConversation_NoBearer_Returns401(t *testing.T) {
	s, _ := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversations", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	s.CreateConversation(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateConversation_ValidBearer_Returns201 verifies that CreateConversation returns 201
// when a valid Bearer token is provided.
func TestCreateConversation_ValidBearer_Returns201(t *testing.T) {
	s, token := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversations", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	s.CreateConversation(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetConversation_NoBearer_Returns401 verifies that GetConversation returns 401
// when credService is configured but no Authorization header is sent.
func TestGetConversation_NoBearer_Returns401(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"
	s, _ := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations/"+convIDStr, nil)

	convID := ConversationId(uuid.MustParse(convIDStr))
	s.GetConversation(w, req, convID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetConversation_ValidBearer_Returns200 verifies that GetConversation returns 200
// when a valid Bearer token is provided.
func TestGetConversation_ValidBearer_Returns200(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"
	s, token := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations/"+convIDStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	convID := ConversationId(uuid.MustParse(convIDStr))
	s.GetConversation(w, req, convID)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteConversation_NoBearer_Returns401 verifies that DeleteConversation returns 401
// when credService is configured but no Authorization header is sent.
func TestDeleteConversation_NoBearer_Returns401(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"
	s, _ := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/conversations/"+convIDStr, nil)

	convID := ConversationId(uuid.MustParse(convIDStr))
	s.DeleteConversation(w, req, convID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDeleteConversation_ValidBearer_Returns200 verifies that DeleteConversation returns 200
// when a valid Bearer token is provided.
func TestDeleteConversation_ValidBearer_Returns200(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"
	s, token := newServerWithCredService()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/conversations/"+convIDStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	convID := ConversationId(uuid.MustParse(convIDStr))
	s.DeleteConversation(w, req, convID)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
