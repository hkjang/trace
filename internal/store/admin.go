package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

type Role struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"isSystem"`
	Permissions []string  `json:"permissions"`
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,email,username,display_name,status,locale,timezone,created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.User{}
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Status, &u.Locale, &u.Timezone, &u.CreatedAt); err != nil {
			return nil, err
		}
		if err := s.loadAuthorization(ctx, &u); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, actor domain.User, email, username, displayName string) (domain.User, error) {
	if !actor.Can("users.manage") {
		return domain.User{}, ErrForbidden
	}
	email = normalizeIdentity(email)
	username = normalizeIdentity(username)
	if email == "" || username == "" || strings.TrimSpace(displayName) == "" {
		return domain.User{}, ErrValidation
	}
	u := domain.User{ID: uuid.New(), Email: email, Username: username, DisplayName: strings.TrimSpace(displayName), Status: "active", Locale: "ko-KR", Timezone: "Asia/Seoul"}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return u, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,email,username,display_name) VALUES($1,$2,$3,$4)`, u.ID, u.Email, u.Username, u.DisplayName); err != nil {
		return domain.User{}, ErrConflict
	}
	var roleID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name='member'`).Scan(&roleID); err != nil {
		return u, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id,assigned_by) VALUES($1,$2,$3)`, u.ID, roleID, actor.ID); err != nil {
		return u, err
	}
	dataKey, err := tracecrypto.GenerateKey()
	if err != nil {
		return u, err
	}
	wrapped, err := s.Vault.Seal(dataKey, "user-data-key:"+u.ID.String()+":1")
	if err != nil {
		return u, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_data_keys(user_id,version,encrypted_key,status,created_by) VALUES($1,1,$2,'active',$3)`, u.ID, wrapped, actor.ID); err != nil {
		return u, err
	}
	return u, tx.Commit(ctx)
}

func (s *Store) SetUserStatus(ctx context.Context, actor domain.User, userID uuid.UUID, status string) error {
	if !actor.Can("users.manage") {
		return ErrForbidden
	}
	if status != "active" && status != "disabled" {
		return ErrValidation
	}
	if actor.ID == userID && status == "disabled" {
		return ErrValidation
	}
	tag, err := s.DB.Exec(ctx, `UPDATE users SET status=$2,updated_at=now() WHERE id=$1`, userID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,description,is_system FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem); err != nil {
			return nil, err
		}
		pRows, err := s.DB.Query(ctx, `SELECT permission_code FROM role_permissions WHERE role_id=$1 ORDER BY permission_code`, r.ID)
		if err != nil {
			return nil, err
		}
		for pRows.Next() {
			var p string
			if err := pRows.Scan(&p); err != nil {
				pRows.Close()
				return nil, err
			}
			r.Permissions = append(r.Permissions, p)
		}
		pRows.Close()
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) UpdateRolePermissions(ctx context.Context, actor domain.User, roleID uuid.UUID, permissions []string) error {
	if !actor.Can("roles.manage") {
		return ErrForbidden
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name string
	if err := tx.QueryRow(ctx, `SELECT name FROM roles WHERE id=$1 FOR UPDATE`, roleID).Scan(&name); err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if name == "administrator" {
		expected := keys(defaultPermissions)
		if len(permissions) != len(expected) {
			return ErrValidation
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, p := range permissions {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code) VALUES($1,$2)`, roleID, p); err != nil {
			return ErrValidation
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) SetUserRoles(ctx context.Context, actor domain.User, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if !actor.Can("users.manage") {
		return ErrForbidden
	}
	if len(roleIDs) == 0 {
		return ErrValidation
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id,assigned_by) VALUES($1,$2,$3)`, userID, roleID, actor.ID); err != nil {
			return ErrValidation
		}
	}
	return tx.Commit(ctx)
}
