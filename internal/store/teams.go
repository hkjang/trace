package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
)

func (s *Store) ListTeams(ctx context.Context, actor domain.User) ([]domain.Team, error) {
	rows, err := s.DB.Query(ctx, `SELECT t.id,t.name,t.description,t.manager_user_id,COALESCE(u.display_name,''),t.created_at,(SELECT count(*) FROM team_members tm WHERE tm.team_id=t.id) FROM teams t LEFT JOIN users u ON u.id=t.manager_user_id WHERE $2 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=t.id AND tm.user_id=$1) ORDER BY t.name`, actor.ID, actor.IsAdmin())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Team{}
	for rows.Next() {
		var item domain.Team
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.ManagerUserID, &item.ManagerName, &item.CreatedAt, &item.MemberCount); err != nil {
			return nil, err
		}
		memberRows, err := s.DB.Query(ctx, `SELECT user_id FROM team_members WHERE team_id=$1 ORDER BY created_at`, item.ID)
		if err != nil {
			return nil, err
		}
		for memberRows.Next() {
			var id uuid.UUID
			if err := memberRows.Scan(&id); err != nil {
				memberRows.Close()
				return nil, err
			}
			item.MemberIDs = append(item.MemberIDs, id)
		}
		memberRows.Close()
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) SaveTeam(ctx context.Context, actor domain.User, teamID *uuid.UUID, name, description string, managerID *uuid.UUID, memberIDs []uuid.UUID) (domain.Team, error) {
	if !actor.Can("users.manage") {
		return domain.Team{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" || managerID == nil {
		return domain.Team{}, ErrValidation
	}
	id := uuid.New()
	if teamID != nil {
		id = *teamID
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Team{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if teamID == nil {
		if _, err := tx.Exec(ctx, `INSERT INTO teams(id,name,description,manager_user_id,created_by) VALUES($1,$2,$3,$4,$5)`, id, name, description, managerID, actor.ID); err != nil {
			return domain.Team{}, ErrConflict
		}
	} else {
		tag, err := tx.Exec(ctx, `UPDATE teams SET name=$2,description=$3,manager_user_id=$4,updated_at=now() WHERE id=$1`, id, name, description, managerID)
		if err != nil {
			return domain.Team{}, ErrConflict
		}
		if tag.RowsAffected() == 0 {
			return domain.Team{}, ErrNotFound
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id=$1`, id); err != nil {
		return domain.Team{}, err
	}
	foundManager := false
	for _, memberID := range memberIDs {
		role := "member"
		if memberID == *managerID {
			role = "manager"
			foundManager = true
		}
		if _, err := tx.Exec(ctx, `INSERT INTO team_members(team_id,user_id,member_role) VALUES($1,$2,$3) ON CONFLICT(team_id,user_id) DO UPDATE SET member_role=excluded.member_role`, id, memberID, role); err != nil {
			return domain.Team{}, ErrValidation
		}
	}
	if !foundManager {
		if _, err := tx.Exec(ctx, `INSERT INTO team_members(team_id,user_id,member_role) VALUES($1,$2,'manager')`, id, managerID); err != nil {
			return domain.Team{}, ErrValidation
		}
		memberIDs = append(memberIDs, *managerID)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'team.save','team',$3)`, uuid.New(), actor.ID, id.String()); err != nil {
		return domain.Team{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Team{}, err
	}
	return domain.Team{ID: id, Name: name, Description: description, ManagerUserID: managerID, MemberIDs: memberIDs, MemberCount: len(memberIDs)}, nil
}
