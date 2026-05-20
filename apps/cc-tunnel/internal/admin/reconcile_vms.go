// Package admin hosts cc-tunnel HTTP endpoints that are not part of the
// public OpenAPI surface — they are invoked by infrastructure (Cloud
// Scheduler, ops tooling) rather than end-user clients. Endpoints are
// registered directly on the root mux in cmd/cc-tunnel/main.go.
package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

// VMReconciler is implemented by any ExecutionProvider that knows how to
// reconcile the GCE VM fleet against the live container-manager state
// (currently only *dockergce.DockerGCEProvider).
type VMReconciler interface {
	ReconcileVMs(ctx context.Context) error
}

// OIDCValidator validates a Google-signed OIDC ID token and returns its
// payload. Pluggable so tests can inject a fake validator without
// reaching Google's JWKs endpoint.
type OIDCValidator func(ctx context.Context, token, audience string) (*idtoken.Payload, error)

// ReconcileVMsHandler validates an OIDC ID token presented in the
// Authorization header against the configured audience + allowed
// service-account email list, then invokes reconciler.ReconcileVMs.
//
// Cloud Scheduler hits this endpoint every 6 hours as the safety-net
// reap path (see adr/2026-05 vm_reap_dual_path.md). The primary reap
// path is the container-manager self-reaper on each VM.
type ReconcileVMsHandler struct {
	Reconciler    VMReconciler
	Audience      string
	AllowedEmails map[string]struct{}
	Validator     OIDCValidator
}

// NewReconcileVMsHandler builds a handler with the production
// idtoken.Validate. audience is typically the Cloud Run service URL;
// allowedEmails is the set of service-account emails authorised to
// invoke this endpoint (the Cloud Scheduler SA).
func NewReconcileVMsHandler(r VMReconciler, audience string, allowedEmails []string) *ReconcileVMsHandler {
	set := make(map[string]struct{}, len(allowedEmails))
	for _, e := range allowedEmails {
		if e = strings.TrimSpace(e); e != "" {
			set[e] = struct{}{}
		}
	}
	return &ReconcileVMsHandler{
		Reconciler:    r,
		Audience:      audience,
		AllowedEmails: set,
		Validator:     idtoken.Validate,
	}
}

func (h *ReconcileVMsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := bearerToken(r.Header.Get("Authorization"))
	if err != nil {
		slog.Warn("reconcile-vms: bad authorization", "err", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	payload, err := h.Validator(r.Context(), token, h.Audience)
	if err != nil {
		slog.Warn("reconcile-vms: oidc validation failed", "err", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email, _ := payload.Claims["email"].(string)
	if _, ok := h.AllowedEmails[email]; !ok {
		slog.Warn("reconcile-vms: email not allowed", "email", email)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.Reconciler.ReconcileVMs(r.Context()); err != nil {
		slog.Error("reconcile-vms: ReconcileVMs failed", "err", err)
		http.Error(w, "reconcile failed", http.StatusInternalServerError)
		return
	}
	slog.Info("reconcile-vms: ok", "email", email)
	w.WriteHeader(http.StatusNoContent)
}

func bearerToken(h string) (string, error) {
	if h == "" {
		return "", errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", errors.New("expected Bearer scheme")
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}
