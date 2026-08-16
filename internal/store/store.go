package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation failed")
)

type Store struct {
	DB    *pgxpool.Pool
	Vault *tracecrypto.Vault
}

func New(db *pgxpool.Pool, vault *tracecrypto.Vault) *Store {
	return &Store{DB: db, Vault: vault}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
