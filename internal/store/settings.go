package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

const SecretPlaceholder = "••••••••"

func (s *Store) GetOIDCSettings(ctx context.Context, includeSecret bool) (domain.OIDCSettings, error) {
	var value []byte
	var encrypted *string
	if err := s.DB.QueryRow(ctx, `SELECT value,encrypted_value FROM system_settings WHERE key='oidc'`).Scan(&value, &encrypted); err != nil {
		return domain.OIDCSettings{}, err
	}
	var settings domain.OIDCSettings
	if err := json.Unmarshal(value, &settings); err != nil {
		return settings, err
	}
	if encrypted != nil {
		if includeSecret {
			plain, err := s.Vault.Open(*encrypted, "system:oidc")
			if err != nil {
				return settings, fmt.Errorf("decrypt OIDC secret: %w", err)
			}
			settings.ClientSecret = string(plain)
		} else {
			settings.ClientSecret = SecretPlaceholder
		}
	}
	return settings, nil
}

func (s *Store) SaveOIDCSettings(ctx context.Context, actor uuid.UUID, settings domain.OIDCSettings) error {
	settings.IssuerURL = strings.TrimRight(strings.TrimSpace(settings.IssuerURL), "/")
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	settings.ClientID = strings.TrimSpace(settings.ClientID)
	if settings.Scopes == "" {
		settings.Scopes = "openid profile email"
	}
	if settings.UsernameClaim == "" {
		settings.UsernameClaim = "preferred_username"
	}
	if settings.EmailClaim == "" {
		settings.EmailClaim = "email"
	}
	if settings.DisplayClaim == "" {
		settings.DisplayClaim = "name"
	}
	if settings.Enabled {
		if _, err := validateHTTPURL(settings.IssuerURL); err != nil {
			return fmt.Errorf("issuer URL: %w", err)
		}
		if settings.ClientID == "" || settings.BaseURL == "" {
			return fmt.Errorf("%w: client ID and public base URL are required", ErrValidation)
		}
		if _, err := validateHTTPURL(settings.BaseURL); err != nil {
			return fmt.Errorf("public base URL: %w", err)
		}
	}
	secret := settings.ClientSecret
	settings.ClientSecret = ""
	value, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if secret != "" && secret != SecretPlaceholder {
		encrypted, err := s.Vault.Seal([]byte(secret), "system:oidc")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE system_settings SET value=$1,encrypted_value=$2,is_secret=true,updated_by=$3,updated_at=now() WHERE key='oidc'`, value, encrypted, actor)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(ctx, `UPDATE system_settings SET value=$1,updated_by=$2,updated_at=now() WHERE key='oidc'`, value, actor)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'settings.update','setting','oidc')`, uuid.New(), actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetAISettings(ctx context.Context, includeSecret bool) (domain.AISettings, error) {
	var value []byte
	var encrypted *string
	if err := s.DB.QueryRow(ctx, `SELECT value,encrypted_value FROM system_settings WHERE key='ai'`).Scan(&value, &encrypted); err != nil {
		return domain.AISettings{}, err
	}
	var settings domain.AISettings
	if err := json.Unmarshal(value, &settings); err != nil {
		return settings, err
	}
	if encrypted != nil {
		if includeSecret {
			plain, err := s.Vault.Open(*encrypted, "system:ai")
			if err != nil {
				return settings, fmt.Errorf("decrypt AI secret: %w", err)
			}
			settings.APIKey = string(plain)
		} else {
			settings.APIKey = SecretPlaceholder
		}
	}
	return settings, nil
}

func (s *Store) SaveAISettings(ctx context.Context, actor uuid.UUID, settings domain.AISettings) error {
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	settings.Model = strings.TrimSpace(settings.Model)
	settings.EmbeddingModel = strings.TrimSpace(settings.EmbeddingModel)
	if settings.Protocol != "responses" && settings.Protocol != "chat_completions" {
		return fmt.Errorf("%w: protocol must be responses or chat_completions", ErrValidation)
	}
	if settings.MaxOutputTokens < 1 || settings.MaxOutputTokens > 262144 {
		return fmt.Errorf("%w: max output tokens must be between 1 and 262144", ErrValidation)
	}
	if settings.ContextWindow < 1 || settings.ContextWindow > 262144 {
		return fmt.Errorf("%w: context window must be between 1 and 262144", ErrValidation)
	}
	if settings.RequestTimeoutSec < 10 || settings.RequestTimeoutSec > 1800 {
		return fmt.Errorf("%w: request timeout must be between 10 and 1800 seconds", ErrValidation)
	}
	if settings.Enabled {
		if _, err := validateHTTPURL(settings.BaseURL); err != nil {
			return fmt.Errorf("AI base URL: %w", err)
		}
		if settings.Model == "" {
			return fmt.Errorf("%w: model is required", ErrValidation)
		}
	}
	secret := settings.APIKey
	settings.APIKey = ""
	value, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if secret != "" && secret != SecretPlaceholder {
		encrypted, err := s.Vault.Seal([]byte(secret), "system:ai")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE system_settings SET value=$1,encrypted_value=$2,is_secret=true,updated_by=$3,updated_at=now() WHERE key='ai'`, value, encrypted, actor)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(ctx, `UPDATE system_settings SET value=$1,updated_by=$2,updated_at=now() WHERE key='ai'`, value, actor)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'settings.update','setting','ai')`, uuid.New(), actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetWorkflowSettings(ctx context.Context) (domain.WorkflowSettings, error) {
	var value []byte
	if err := s.DB.QueryRow(ctx, `SELECT value FROM system_settings WHERE key='workflow'`).Scan(&value); err != nil {
		return domain.WorkflowSettings{}, err
	}
	var settings domain.WorkflowSettings
	return settings, json.Unmarshal(value, &settings)
}

func (s *Store) SaveWorkflowSettings(ctx context.Context, actor uuid.UUID, settings domain.WorkflowSettings) error {
	value, _ := json.Marshal(settings)
	command, err := s.DB.Exec(ctx, `UPDATE system_settings SET value=$1,updated_by=$2,updated_at=now() WHERE key='workflow'`, value, actor)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetBrandingSettings(ctx context.Context) (domain.BrandingSettings, error) {
	var value []byte
	if err := s.DB.QueryRow(ctx, `SELECT value FROM system_settings WHERE key='branding'`).Scan(&value); err != nil {
		return domain.BrandingSettings{}, err
	}
	var settings domain.BrandingSettings
	return settings, json.Unmarshal(value, &settings)
}

func (s *Store) SaveBrandingSettings(ctx context.Context, actor uuid.UUID, settings domain.BrandingSettings) error {
	settings.ServiceName = strings.TrimSpace(settings.ServiceName)
	if settings.ServiceName == "" {
		return ErrValidation
	}
	value, _ := json.Marshal(settings)
	_, err := s.DB.Exec(ctx, `UPDATE system_settings SET value=$1,updated_by=$2,updated_at=now() WHERE key='branding'`, value, actor)
	return err
}

func validateHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, ErrValidation
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("must use http or https")
	}
	if parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("must contain a host and no embedded credentials")
	}
	return parsed, nil
}

func settingExists(ctx context.Context, s *Store, key string) bool {
	var ok bool
	_ = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM system_settings WHERE key=$1)`, key).Scan(&ok)
	return ok
}

var _ = pgx.ErrNoRows
