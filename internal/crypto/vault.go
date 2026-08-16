package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const envelopeVersion byte = 1

// Vault provides authenticated encryption for system secrets and per-user data
// encryption keys. Ciphertext is versioned so the envelope format can evolve.
type Vault struct {
	aead cipher.AEAD
}

func NewVault(masterKey []byte) (*Vault, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Seal(plain []byte, purpose string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := v.aead.Seal(nil, nonce, plain, []byte(purpose))
	payload := make([]byte, 1+len(nonce)+len(sealed))
	payload[0] = envelopeVersion
	copy(payload[1:], nonce)
	copy(payload[1+len(nonce):], sealed)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (v *Vault) Open(encoded string, purpose string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(payload) < 1+v.aead.NonceSize()+v.aead.Overhead() || payload[0] != envelopeVersion {
		return nil, errors.New("invalid ciphertext envelope")
	}
	nonce := payload[1 : 1+v.aead.NonceSize()]
	plain, err := v.aead.Open(nil, nonce, payload[1+v.aead.NonceSize():], []byte(purpose))
	if err != nil {
		return nil, errors.New("ciphertext authentication failed")
	}
	return plain, nil
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}
