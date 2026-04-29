package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	KeyLength   = 32 // AES-256
	NonceLength = 12 // GCM standard
)

var (
	ErrInvalidKeyLength = errors.New("encryption key must be 32 bytes")
	ErrDecryptionFailed = errors.New("decryption failed (auth tag mismatch or corrupted ciphertext)")
)

type Encryptor struct {
	aead       cipher.AEAD
	keyVersion int
}

func NewEncryptor(key []byte, keyVersion int) (*Encryptor, error) {
	if len(key) != KeyLength {
		return nil, ErrInvalidKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	return &Encryptor{aead: aead, keyVersion: keyVersion}, nil
}

// Seal encrypts plaintext. AAD is bound to username to prevent cross-user nonce reuse attacks.
func (e *Encryptor) Seal(plaintext []byte, username string) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, NonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("rand.Read: %w", err)
	}
	ciphertext = e.aead.Seal(nil, nonce, plaintext, []byte(username))
	return ciphertext, nonce, nil
}

// Open decrypts ciphertext using the nonce and username as AAD.
func (e *Encryptor) Open(ciphertext, nonce []byte, username string) ([]byte, error) {
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, []byte(username))
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

func (e *Encryptor) KeyVersion() int { return e.keyVersion }
