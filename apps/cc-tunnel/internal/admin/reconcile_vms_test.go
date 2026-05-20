package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/api/idtoken"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/admin"
)

type stubReconciler struct {
	calls atomic.Int32
	err   error
}

func (s *stubReconciler) ReconcileVMs(_ context.Context) error {
	s.calls.Add(1)
	return s.err
}

func validator(email string, err error) admin.OIDCValidator {
	return func(_ context.Context, _, _ string) (*idtoken.Payload, error) {
		if err != nil {
			return nil, err
		}
		return &idtoken.Payload{Claims: map[string]any{"email": email}}, nil
	}
}

func newHandler(t *testing.T, rec admin.VMReconciler, v admin.OIDCValidator, allowed ...string) *admin.ReconcileVMsHandler {
	t.Helper()
	h := admin.NewReconcileVMsHandler(rec, "https://example.run.app", allowed)
	h.Validator = v
	return h
}

func req(t *testing.T, method, auth string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, "/internal/reconcile-vms", strings.NewReader(""))
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func TestReconcileHandler_HappyPath(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("scheduler@proj.iam.gserviceaccount.com", nil),
		"scheduler@proj.iam.gserviceaccount.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, "Bearer test-token"))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%q", rr.Code, rr.Body.String())
	}
	if got := rec.calls.Load(); got != 1 {
		t.Fatalf("ReconcileVMs called %d times, want 1", got)
	}
}

func TestReconcileHandler_RejectsWrongMethod(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("scheduler@x", nil), "scheduler@x")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodGet, "Bearer t"))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
	if rec.calls.Load() != 0 {
		t.Fatalf("ReconcileVMs should not be called on wrong method")
	}
}

func TestReconcileHandler_RejectsMissingAuth(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("scheduler@x", nil), "scheduler@x")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestReconcileHandler_RejectsBadScheme(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("scheduler@x", nil), "scheduler@x")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, "Basic dXNlcjpwYXNz"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestReconcileHandler_RejectsInvalidToken(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("", errors.New("bad sig")), "scheduler@x")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, "Bearer x"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rec.calls.Load() != 0 {
		t.Fatalf("ReconcileVMs must not run on bad token")
	}
}

func TestReconcileHandler_RejectsUnauthorizedEmail(t *testing.T) {
	rec := &stubReconciler{}
	h := newHandler(t, rec, validator("attacker@evil.example", nil),
		"scheduler@proj.iam.gserviceaccount.com")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, "Bearer x"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if rec.calls.Load() != 0 {
		t.Fatalf("ReconcileVMs must not run for non-allowed email")
	}
}

func TestReconcileHandler_ReconcileError(t *testing.T) {
	rec := &stubReconciler{err: errors.New("boom")}
	h := newHandler(t, rec, validator("scheduler@x", nil), "scheduler@x")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req(t, http.MethodPost, "Bearer x"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}
