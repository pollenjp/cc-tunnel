package api

import (
	"net/http"
	"strings"
	"sync"
)

// AppSession is an in-memory session store for mock authentication.
// It maps Bearer tokens to AppUser values. Safe for concurrent use.
type AppSession struct {
	mu    sync.RWMutex
	store map[string]AppUser // token → user
}

func newAppSession() *AppSession {
	return &AppSession{store: make(map[string]AppUser)}
}

func (s *AppSession) set(token string, user AppUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[token] = user
}

func (s *AppSession) get(token string) (AppUser, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.store[token]
	return u, ok
}

func (s *AppSession) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, token)
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	token, found := strings.CutPrefix(auth, "Bearer ")
	if !found || token == "" {
		return "", false
	}
	return token, true
}

// requireAppAuthIfEnabled enforces app-level Bearer auth when credService is
// configured. It writes the appropriate 401 response and returns ok=false on
// rejection. When credService is nil, auth is bypassed and ok=true is returned
// with a zero AppUser.
//
// Response shape on failure intentionally matches the legacy handlers:
//   - missing/empty bearer → Error{Error: "unauthorized"}   (writeError)
//   - unknown token        → AppAuthError{Message: "unauthorized"}
func (h *Server) requireAppAuthIfEnabled(w http.ResponseWriter, r *http.Request) (AppUser, bool) {
	if h.credService == nil {
		return AppUser{}, true
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return AppUser{}, false
	}
	user, found := h.session.get(token)
	if !found {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return AppUser{}, false
	}
	return user, true
}
