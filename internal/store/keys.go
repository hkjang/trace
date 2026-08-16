package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) activeDataKey(ctx context.Context, userID uuid.UUID) ([]byte, int, error) {
	var wrapped string
	var version int
	if err := s.DB.QueryRow(ctx, `SELECT encrypted_key,version FROM user_data_keys WHERE user_id=$1 AND status='active'`, userID).Scan(&wrapped, &version); err != nil {
		return nil, 0, err
	}
	key, err := s.Vault.Open(wrapped, fmt.Sprintf("user-data-key:%s:%d", userID, version))
	return key, version, err
}

func (s *Store) ListPersonalKeys(ctx context.Context, userID uuid.UUID) ([]domain.PersonalKey, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,kind,permissions,status,expires_at,last_rotated_at,created_at FROM personal_keys WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.PersonalKey{}
	for rows.Next() {
		var item domain.PersonalKey
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Kind, &raw, &item.Status, &item.ExpiresAt, &item.LastRotatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.Permissions); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CreatePersonalKey(ctx context.Context, userID uuid.UUID, name, kind, value string, permissions []string, expires *time.Time) (domain.PersonalKey, error) {
	if name == "" || value == "" || len(permissions) == 0 {
		return domain.PersonalKey{}, ErrValidation
	}
	dataKey, version, err := s.activeDataKey(ctx, userID)
	if err != nil {
		return domain.PersonalKey{}, err
	}
	vault, err := tracecrypto.NewVault(dataKey)
	if err != nil {
		return domain.PersonalKey{}, err
	}
	item := domain.PersonalKey{ID: uuid.New(), Name: name, Kind: kind, Permissions: permissions, Status: "active", ExpiresAt: expires, LastRotatedAt: time.Now().UTC(), CreatedAt: time.Now().UTC()}
	sealed, err := vault.Seal([]byte(value), "personal-key:"+item.ID.String())
	if err != nil {
		return item, err
	}
	raw, _ := json.Marshal(permissions)
	_, err = s.DB.Exec(ctx, `INSERT INTO personal_keys(id,user_id,name,kind,encrypted_value,data_key_version,permissions,expires_at,last_rotated_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`, item.ID, userID, name, kind, sealed, version, raw, expires, item.LastRotatedAt, item.CreatedAt)
	if err != nil {
		return domain.PersonalKey{}, ErrConflict
	}
	return item, nil
}

func (s *Store) RotatePersonalKey(ctx context.Context, userID, keyID uuid.UUID, newValue string) (domain.PersonalKey, error) {
	if newValue == "" {
		return domain.PersonalKey{}, ErrValidation
	}
	dataKey, version, err := s.activeDataKey(ctx, userID)
	if err != nil {
		return domain.PersonalKey{}, err
	}
	vault, _ := tracecrypto.NewVault(dataKey)
	sealed, err := vault.Seal([]byte(newValue), "personal-key:"+keyID.String())
	if err != nil {
		return domain.PersonalKey{}, err
	}
	var item domain.PersonalKey
	var raw []byte
	err = s.DB.QueryRow(ctx, `UPDATE personal_keys SET encrypted_value=$3,data_key_version=$4,last_rotated_at=now(),updated_at=now(),status='active' WHERE id=$1 AND user_id=$2 RETURNING id,name,kind,permissions,status,expires_at,last_rotated_at,created_at`, keyID, userID, sealed, version).Scan(&item.ID, &item.Name, &item.Kind, &raw, &item.Status, &item.ExpiresAt, &item.LastRotatedAt, &item.CreatedAt)
	if err != nil {
		return domain.PersonalKey{}, ErrNotFound
	}
	_ = json.Unmarshal(raw, &item.Permissions)
	return item, nil
}

func (s *Store) UpdatePersonalKeyPermissions(ctx context.Context, userID, keyID uuid.UUID, permissions []string) error {
	if len(permissions) == 0 {
		return ErrValidation
	}
	raw, _ := json.Marshal(permissions)
	tag, err := s.DB.Exec(ctx, `UPDATE personal_keys SET permissions=$3,updated_at=now() WHERE id=$1 AND user_id=$2`, keyID, userID, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RevokePersonalKey(ctx context.Context, userID, keyID uuid.UUID) error {
	tag, err := s.DB.Exec(ctx, `UPDATE personal_keys SET status='revoked',updated_at=now() WHERE id=$1 AND user_id=$2`, keyID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateUserDataKey creates a fresh per-user DEK and re-encrypts every active
// personal secret in one transaction. The root ENCRYPTION_KEY never leaves the
// process and old DEKs are retained only as wrapped, retired metadata.
func (s *Store) RotateUserDataKey(ctx context.Context, actor, target domain.User) error {
	if actor.ID != target.ID && !actor.Can("keys.manage_all") {
		return ErrForbidden
	}
	oldKey, oldVersion, err := s.activeDataKey(ctx, target.ID)
	if err != nil {
		return err
	}
	oldVault, _ := tracecrypto.NewVault(oldKey)
	newKey, err := tracecrypto.GenerateKey()
	if err != nil {
		return err
	}
	newVersion := oldVersion + 1
	newVault, _ := tracecrypto.NewVault(newKey)
	wrapped, err := s.Vault.Seal(newKey, fmt.Sprintf("user-data-key:%s:%d", target.ID, newVersion))
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT id,encrypted_value FROM personal_keys WHERE user_id=$1 FOR UPDATE`, target.ID)
	if err != nil {
		return err
	}
	type secret struct {
		id     uuid.UUID
		sealed string
	}
	var secrets []secret
	for rows.Next() {
		var v secret
		if err := rows.Scan(&v.id, &v.sealed); err != nil {
			rows.Close()
			return err
		}
		secrets = append(secrets, v)
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `UPDATE user_data_keys SET status='retired',retired_at=now() WHERE user_id=$1 AND status='active'`, target.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_data_keys(user_id,version,encrypted_key,status,created_by) VALUES($1,$2,$3,'active',$4)`, target.ID, newVersion, wrapped, actor.ID); err != nil {
		return err
	}
	for _, entry := range secrets {
		plain, err := oldVault.Open(entry.sealed, "personal-key:"+entry.id.String())
		if err != nil {
			return err
		}
		sealed, err := newVault.Seal(plain, "personal-key:"+entry.id.String())
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE personal_keys SET encrypted_value=$2,data_key_version=$3,updated_at=now() WHERE id=$1`, entry.id, sealed, newVersion); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'key.data.rotate','user',$3,$4)`, uuid.New(), actor.ID, target.ID.String(), []byte(fmt.Sprintf(`{"from":%d,"to":%d}`, oldVersion, newVersion)))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var _ = pgx.ErrNoRows
