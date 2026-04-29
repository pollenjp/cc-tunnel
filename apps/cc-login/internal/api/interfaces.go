package api

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/auth"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/credential"
)

// PTYManager manages the PTY-based claude /auth login session.
type PTYManager interface {
	StartLogin(ctx context.Context, method string) (auth.LoginResponse, error)
	GetOutput(since int) (data string, cursor int, status auth.LoginStatus)
	SubmitInput(input string) error
	Cancel()
	ReadCredentials() ([]byte, error)
	ClearLocalCredentials() error
	Status() auth.LoginStatus
}

// CredentialStore handles persistence of encrypted credentials.
type CredentialStore interface {
	Upsert(ctx context.Context, c *credential.Credential) error
	GetByUsername(ctx context.Context, username string) (*credential.Credential, error)
	MarkInvalid(ctx context.Context, username string) error
	Delete(ctx context.Context, username string) error
	UpdateLastValidated(ctx context.Context, username string) error
}
