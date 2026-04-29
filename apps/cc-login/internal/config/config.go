package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	Port             string // PORT (default: 9092)
	DatabaseURL      string // DATABASE_URL
	EncryptionKey    []byte // CC_LOGIN_ENCRYPTION_KEY decoded (32 bytes)
	CCTunnelURL      string // CC_TUNNEL_URL (default: http://cc-tunnel:8080)
}

func Load() (*Config, error) {
	keyB64 := os.Getenv("CC_LOGIN_ENCRYPTION_KEY")
	if keyB64 == "" {
		return nil, errors.New("CC_LOGIN_ENCRYPTION_KEY environment variable is required")
	}

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("CC_LOGIN_ENCRYPTION_KEY base64 decode failed: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("CC_LOGIN_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://cctunnel:cctunnel_dev@localhost:5432/cctunnel?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9092"
	}

	ccTunnelURL := os.Getenv("CC_TUNNEL_URL")
	if ccTunnelURL == "" {
		ccTunnelURL = "http://cc-tunnel:8080"
	}

	return &Config{
		Port:          port,
		DatabaseURL:   dbURL,
		EncryptionKey: key,
		CCTunnelURL:   ccTunnelURL,
	}, nil
}
