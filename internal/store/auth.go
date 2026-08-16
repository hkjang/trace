package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) AuthenticateLocal(ctx context.Context, identity, password string) (domain.User, error) {
	var user domain.User
	var passwordHash *string
	err := s.DB.QueryRow(ctx, `SELECT id,email,username,display_name,status,locale,timezone,password_hash,created_at FROM users WHERE (lower(email)=$1 OR lower(username)=$1) AND status='active'`, normalizeIdentity(identity)).Scan(
		&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.Status, &user.Locale, &user.Timezone, &passwordHash, &user.CreatedAt,
	)
	if err != nil || passwordHash == nil || bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(password)) != nil {
		return domain.User{}, ErrUnauthorized
	}
	if err := s.loadAuthorization(ctx, &user); err != nil {
		return domain.User{}, err
	}
	_, _ = s.DB.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, user.ID)
	return user, nil
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var user domain.User
	err := s.DB.QueryRow(ctx, `SELECT id,email,username,display_name,status,locale,timezone,created_at FROM users WHERE id=$1 AND status='active'`, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.Status, &user.Locale, &user.Timezone, &user.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	if err := s.loadAuthorization(ctx, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) loadAuthorization(ctx context.Context, user *domain.User) error {
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT r.name, rp.permission_code FROM user_roles ur JOIN roles r ON r.id=ur.role_id LEFT JOIN role_permissions rp ON rp.role_id=r.id WHERE ur.user_id=$1 ORDER BY r.name,rp.permission_code`, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	seenRoles := map[string]bool{}
	seenPermissions := map[string]bool{}
	for rows.Next() {
		var role string
		var permission *string
		if err := rows.Scan(&role, &permission); err != nil {
			return err
		}
		if !seenRoles[role] {
			user.Roles = append(user.Roles, role)
			seenRoles[role] = true
		}
		if permission != nil && !seenPermissions[*permission] {
			user.Permissions = append(user.Permissions, *permission)
			seenPermissions[*permission] = true
		}
	}
	return rows.Err()
}

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, method, userAgent, ip string, ttl time.Duration) (string, time.Time, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(ttl)
	_, err = s.DB.Exec(ctx, `INSERT INTO sessions(id,user_id,token_hash,auth_method,user_agent,ip_address,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, uuid.New(), userID, tokenHash(token), method, userAgent, ip, expires)
	return token, expires, err
}

func (s *Store) UserFromSession(ctx context.Context, token string) (domain.User, error) {
	var userID uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT user_id FROM sessions WHERE token_hash=$1 AND expires_at>now()`, tokenHash(token)).Scan(&userID)
	if err != nil {
		return domain.User{}, ErrUnauthorized
	}
	return s.UserByID(ctx, userID)
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash(token))
	return err
}

func (s *Store) CreateAPIToken(ctx context.Context, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (domain.APIToken, string, error) {
	if name == "" || len(scopes) == 0 {
		return domain.APIToken{}, "", ErrValidation
	}
	random, err := randomToken(32)
	if err != nil {
		return domain.APIToken{}, "", err
	}
	plain := "trc_" + random
	prefix := plain[:12]
	item := domain.APIToken{ID: uuid.New(), Name: name, Prefix: prefix, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()}
	_, err = s.DB.Exec(ctx, `INSERT INTO api_tokens(id,user_id,name,token_prefix,token_hash,scopes,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, userID, name, prefix, tokenHash(plain), scopes, expiresAt, item.CreatedAt)
	if err != nil {
		return domain.APIToken{}, "", fmt.Errorf("create API token: %w", err)
	}
	return item, plain, nil
}

func (s *Store) AuthenticateAPIToken(ctx context.Context, token string) (domain.User, []string, error) {
	var userID uuid.UUID
	var scopes []string
	err := s.DB.QueryRow(ctx, `UPDATE api_tokens SET last_used_at=now() WHERE token_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()) RETURNING user_id,scopes`, tokenHash(token)).Scan(&userID, &scopes)
	if err != nil {
		return domain.User{}, nil, ErrUnauthorized
	}
	user, err := s.UserByID(ctx, userID)
	return user, scopes, err
}

func (s *Store) ListAPITokens(ctx context.Context, userID uuid.UUID) ([]domain.APIToken, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,token_prefix,scopes,last_used_at,expires_at,created_at FROM api_tokens WHERE user_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.APIToken{}
	for rows.Next() {
		var item domain.APIToken
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &item.Scopes, &item.LastUsedAt, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) RevokeAPIToken(ctx context.Context, userID, tokenID uuid.UUID) error {
	command, err := s.DB.Exec(ctx, `UPDATE api_tokens SET revoked_at=now() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, tokenID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func randomToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
