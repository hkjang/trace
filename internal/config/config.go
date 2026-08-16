package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	EnvPostgresDSN            = "POSTGRES_DSN"
	EnvBootstrapAdmin         = "BOOTSTRAP_ADMIN"
	EnvBootstrapAdminPassword = "BOOTSTRAP_ADMIN_PASSWORD"
	EnvEncryptionKey          = "ENCRYPTION_KEY"
)

// Config is intentionally restricted to the four deployment inputs approved for
// Trace. Network, OIDC, AI, workflow, and policy settings live in PostgreSQL and
// are managed from the administrator UI.
type Config struct {
	PostgresDSN            string
	BootstrapAdmin         string
	BootstrapAdminPassword string
	EncryptionKey          []byte
	ListenAddress          string
}

func Load() (Config, error) {
	cfg := Config{
		PostgresDSN:            strings.TrimSpace(os.Getenv(EnvPostgresDSN)),
		BootstrapAdmin:         strings.TrimSpace(os.Getenv(EnvBootstrapAdmin)),
		BootstrapAdminPassword: os.Getenv(EnvBootstrapAdminPassword),
		ListenAddress:          ":8080",
	}

	var missing []string
	if cfg.PostgresDSN == "" {
		missing = append(missing, EnvPostgresDSN)
	}
	if cfg.BootstrapAdmin == "" {
		missing = append(missing, EnvBootstrapAdmin)
	}
	if cfg.BootstrapAdminPassword == "" {
		missing = append(missing, EnvBootstrapAdminPassword)
	}
	keyValue := strings.TrimSpace(os.Getenv(EnvEncryptionKey))
	if keyValue == "" {
		missing = append(missing, EnvEncryptionKey)
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if len(cfg.BootstrapAdminPassword) < 12 {
		return Config{}, errors.New("BOOTSTRAP_ADMIN_PASSWORD must contain at least 12 characters")
	}

	key, err := decodeKey(keyValue)
	if err != nil {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}
	cfg.EncryptionKey = key
	return cfg, nil
}

func decodeKey(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len([]byte(value)) == 32 {
		return []byte(value), nil
	}
	return nil, errors.New("must be exactly 32 bytes encoded as base64, hex, or raw text")
}
