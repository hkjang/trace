package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	tracecrypto "github.com/hkjang/trace/internal/crypto"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var defaultPermissions = map[string]string{
	"admin.access":      "Open service administration pages",
	"settings.manage":   "Change service settings and encrypted secrets",
	"users.manage":      "Create, disable, and assign roles to users",
	"roles.manage":      "Change role permission mappings",
	"keys.manage_own":   "Create and rotate personal keys",
	"keys.manage_all":   "Administer all users' key lifecycle metadata",
	"decisions.read":    "Read visible decisions",
	"decisions.write":   "Create and edit owned decisions",
	"decisions.approve": "Approve or reject team decisions",
	"ai.use":            "Invoke configured AI providers",
	"tokens.manage":     "Create and revoke personal API tokens",
	"audit.read":        "Read audit events",
}

func (s *Store) Bootstrap(ctx context.Context, identity, password string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for code, description := range defaultPermissions {
		if _, err := tx.Exec(ctx, `INSERT INTO permissions(code, description) VALUES($1,$2) ON CONFLICT(code) DO UPDATE SET description=excluded.description`, code, description); err != nil {
			return fmt.Errorf("seed permission %s: %w", code, err)
		}
	}
	roles := []struct {
		name, description string
		permissions       []string
	}{
		{"administrator", "Service administrators", keys(defaultPermissions)},
		{"team_manager", "Team review and approval managers", []string{"decisions.read", "decisions.write", "decisions.approve", "keys.manage_own", "ai.use", "tokens.manage"}},
		{"member", "Standard Trace users", []string{"decisions.read", "decisions.write", "keys.manage_own", "ai.use", "tokens.manage"}},
	}
	roleIDs := make(map[string]uuid.UUID, len(roles))
	for _, role := range roles {
		var roleID uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO roles(id,name,description,is_system) VALUES($1,$2,$3,true) ON CONFLICT(name) DO UPDATE SET description=excluded.description RETURNING id`, uuid.New(), role.name, role.description).Scan(&roleID)
		if err != nil {
			return fmt.Errorf("seed role %s: %w", role.name, err)
		}
		roleIDs[role.name] = roleID
		if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
			return err
		}
		for _, permission := range role.permissions {
			if _, err := tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code) VALUES($1,$2)`, roleID, permission); err != nil {
				return err
			}
		}
	}

	normalized := normalizeIdentity(identity)
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email)=$1 OR lower(username)=$1 LIMIT 1`, normalized).Scan(&userID)
	if err == pgx.ErrNoRows {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		userID = uuid.New()
		email := normalized
		username := normalized
		if at := strings.Index(normalized, "@"); at > 0 {
			username = normalized[:at]
		} else {
			email = normalized + "@trace.local"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,email,username,display_name,password_hash) VALUES($1,$2,$3,$4,$5)`, userID, email, username, "Trace Administrator", string(hash)); err != nil {
			return fmt.Errorf("create bootstrap administrator: %w", err)
		}
		dataKey, err := tracecrypto.GenerateKey()
		if err != nil {
			return err
		}
		wrapped, err := s.Vault.Seal(dataKey, "user-data-key:"+userID.String()+":1")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_data_keys(user_id,version,encrypted_key,status,created_by) VALUES($1,1,$2,'active',$1)`, userID, wrapped); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("find bootstrap administrator: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id,assigned_by) VALUES($1,$2,$1) ON CONFLICT DO NOTHING`, userID, roleIDs["administrator"]); err != nil {
		return err
	}

	defaults := []struct {
		key, value string
	}{
		{"branding", `{"serviceName":"Trace","tagline":"Remember why you decided."}`},
		{"oidc", `{"enabled":false,"issuerUrl":"","clientId":"","scopes":"openid profile email","usernameClaim":"preferred_username","emailClaim":"email","displayClaim":"name","autoProvision":true,"baseUrl":""}`},
		{"ai", `{"enabled":false,"providerName":"OpenAI compatible","baseUrl":"https://api.openai.com/v1","protocol":"responses","model":"","maxOutputTokens":4096,"contextWindow":262144,"requestTimeoutSec":300,"systemPrompt":"AI clarifies the user's thinking; it does not replace it."}`},
		{"workflow", `{"approvalRequired":false,"requireTeamManager":true}`},
		{"security", `{"sessionHours":12,"allowLocalLogin":true}`},
	}
	for _, setting := range defaults {
		if _, err := tx.Exec(ctx, `INSERT INTO system_settings(key,value,updated_by) VALUES($1,$2::jsonb,$3) ON CONFLICT(key) DO NOTHING`, setting.key, setting.value, userID); err != nil {
			return fmt.Errorf("seed setting %s: %w", setting.key, err)
		}
	}

	return tx.Commit(ctx)
}

func keys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}
