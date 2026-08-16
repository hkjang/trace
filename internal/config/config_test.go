package config

import (
	"encoding/base64"
	"testing"
)

func TestLoadAcceptsFourRequiredVariables(t *testing.T) {
	t.Setenv(EnvPostgresDSN, "postgres://trace:test@db/trace")
	t.Setenv(EnvBootstrapAdmin, "admin@example.test")
	t.Setenv(EnvBootstrapAdminPassword, "correct-horse-battery-staple")
	t.Setenv(EnvEncryptionKey, base64.StdEncoding.EncodeToString(make([]byte, 32)))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.EncryptionKey) != 32 || cfg.ListenAddress != ":8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsWeakBootstrapPassword(t *testing.T) {
	t.Setenv(EnvPostgresDSN, "postgres://trace:test@db/trace")
	t.Setenv(EnvBootstrapAdmin, "admin")
	t.Setenv(EnvBootstrapAdminPassword, "short")
	t.Setenv(EnvEncryptionKey, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}
