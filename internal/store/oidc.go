package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateOAuthState(ctx context.Context, state, nonce, verifier, returnTo string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO oauth_states(state_hash,nonce,code_verifier,return_to,expires_at) VALUES($1,$2,$3,$4,$5)`, tokenHash(state), nonce, verifier, returnTo, time.Now().UTC().Add(10*time.Minute))
	return err
}

func (s *Store) ConsumeOAuthState(ctx context.Context, state string) (nonce, verifier, returnTo string, err error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `DELETE FROM oauth_states WHERE state_hash=$1 AND expires_at>now() RETURNING nonce,code_verifier,return_to`, tokenHash(state)).Scan(&nonce, &verifier, &returnTo)
	if err != nil {
		return "", "", "", ErrUnauthorized
	}
	err = tx.Commit(ctx)
	return
}

func (s *Store) ResolveOIDCUser(ctx context.Context, issuer, subject, email, username, displayName string, claims map[string]any, autoProvision bool) (domain.User, error) {
	var userID uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT user_id FROM oidc_identities WHERE issuer=$1 AND subject=$2`, issuer, subject).Scan(&userID)
	if err == nil {
		return s.UserByID(ctx, userID)
	}
	if err != pgx.ErrNoRows {
		return domain.User{}, err
	}
	email = normalizeIdentity(email)
	username = normalizeIdentity(username)
	if email == "" {
		return domain.User{}, ErrValidation
	}
	if username == "" {
		username = email
	}
	if displayName == "" {
		displayName = username
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email)=$1`, email).Scan(&userID)
	created := false
	if err == pgx.ErrNoRows {
		if !autoProvision {
			return domain.User{}, ErrForbidden
		}
		userID = uuid.New()
		created = true
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,email,username,display_name) VALUES($1,$2,$3,$4)`, userID, email, username, displayName); err != nil {
			return domain.User{}, ErrConflict
		}
		var roleID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name='member'`).Scan(&roleID); err != nil {
			return domain.User{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, userID, roleID); err != nil {
			return domain.User{}, err
		}
		dataKey, err := tracecrypto.GenerateKey()
		if err != nil {
			return domain.User{}, err
		}
		wrapped, err := s.Vault.Seal(dataKey, "user-data-key:"+userID.String()+":1")
		if err != nil {
			return domain.User{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_data_keys(user_id,version,encrypted_key,status) VALUES($1,1,$2,'active')`, userID, wrapped); err != nil {
			return domain.User{}, err
		}
	} else if err != nil {
		return domain.User{}, err
	}
	raw, _ := json.Marshal(claims)
	if _, err := tx.Exec(ctx, `INSERT INTO oidc_identities(issuer,subject,user_id,claims) VALUES($1,$2,$3,$4)`, issuer, subject, userID, raw); err != nil {
		return domain.User{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET last_login_at=now(),display_name=CASE WHEN $2<>'' THEN $2 ELSE display_name END WHERE id=$1`, userID, displayName); err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	_ = created
	return s.UserByID(ctx, userID)
}
