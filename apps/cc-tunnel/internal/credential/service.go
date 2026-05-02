package credential

import (
	"context"
	"errors"
)

var (
	// ErrCredentialsInvalid is returned when stored credentials have is_valid=FALSE.
	ErrCredentialsInvalid = errors.New("credentials are invalid (re-login required)")
	// ErrKeyVersionMismatch is returned when the stored key_version does not match the encryptor's key version.
	ErrKeyVersionMismatch = errors.New("key version mismatch")
)

type credentialRepo interface {
	GetByUsername(ctx context.Context, username string) (*Credential, error)
	MarkInvalid(ctx context.Context, username string) error
	Upsert(ctx context.Context, c *Credential) error
}

// CredentialService fetches and decrypts user credentials from the DB.
type CredentialService struct {
	repo      credentialRepo
	encryptor *Encryptor
}

func NewCredentialService(repo credentialRepo, encryptor *Encryptor) *CredentialService {
	return &CredentialService{repo: repo, encryptor: encryptor}
}

// FetchAndDecrypt retrieves the encrypted credential for username and decrypts it.
// Returns ErrNotFound if no credential exists, ErrCredentialsInvalid if is_valid=FALSE,
// ErrKeyVersionMismatch if the key version differs, or ErrDecryptionFailed on decryption error.
func (s *CredentialService) FetchAndDecrypt(ctx context.Context, username string) ([]byte, error) {
	cred, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if !cred.IsValid {
		return nil, ErrCredentialsInvalid
	}
	if cred.KeyVersion != s.encryptor.KeyVersion() {
		return nil, ErrKeyVersionMismatch
	}
	return s.encryptor.Open(cred.EncryptedData, cred.Nonce, username)
}

// MarkInvalid marks the credentials for username as invalid in the DB.
func (s *CredentialService) MarkInvalid(ctx context.Context, username string) error {
	return s.repo.MarkInvalid(ctx, username)
}

// StoreCredential encrypts credJSON and upserts it into the credentials store.
func (s *CredentialService) StoreCredential(ctx context.Context, username, credJSON string) error {
	ciphertext, nonce, err := s.encryptor.Seal([]byte(credJSON), username)
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, &Credential{
		Username:      username,
		EncryptedData: ciphertext,
		Nonce:         nonce,
		KeyVersion:    s.encryptor.KeyVersion(),
	})
}
