package store

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
)

func (s *Store) SubmitDecisionForApproval(ctx context.Context, actor domain.User, decisionID uuid.UUID, note string) (domain.ApprovalRequest, error) {
	workflow, err := s.GetWorkflowSettings(ctx)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if !workflow.ApprovalRequired {
		return domain.ApprovalRequest{}, ErrValidation
	}
	decision, err := s.GetDecision(ctx, actor, decisionID, nil)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if decision.OwnerID != actor.ID && !actor.IsAdmin() {
		return domain.ApprovalRequest{}, ErrForbidden
	}
	if decision.WorkflowState != "draft" && decision.WorkflowState != "rejected" {
		return domain.ApprovalRequest{}, ErrConflict
	}
	var reviewerID *uuid.UUID
	if decision.TeamID != nil {
		var id uuid.UUID
		if err := s.DB.QueryRow(ctx, `SELECT COALESCE(t.manager_user_id, (SELECT tm.user_id FROM team_members tm WHERE tm.team_id=t.id AND tm.member_role='manager' ORDER BY tm.created_at LIMIT 1)) FROM teams t WHERE t.id=$1`, decision.TeamID).Scan(&id); err == nil {
			reviewerID = &id
		}
	}
	if workflow.RequireTeamManager && reviewerID == nil {
		return domain.ApprovalRequest{}, ErrValidation
	}
	item := domain.ApprovalRequest{ID: uuid.New(), DecisionID: decisionID, DecisionTitle: decision.Title, RequesterID: actor.ID, RequesterName: actor.DisplayName, ReviewerID: reviewerID, State: "pending", RequestNote: strings.TrimSpace(note), RequestedAt: time.Now().UTC()}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO approval_requests(id,decision_id,requester_id,reviewer_id,state,request_note,requested_at) VALUES($1,$2,$3,$4,'pending',$5,$6)`, item.ID, decisionID, actor.ID, reviewerID, item.RequestNote, item.RequestedAt); err != nil {
		return item, ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO approval_events(id,approval_request_id,actor_id,action,note) VALUES($1,$2,$3,'requested',$4)`, uuid.New(), item.ID, actor.ID, item.RequestNote); err != nil {
		return item, err
	}
	if _, err := tx.Exec(ctx, `UPDATE decisions SET workflow_state='pending',updated_at=now(),version=version+1 WHERE id=$1`, decisionID); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (s *Store) ReviewApproval(ctx context.Context, actor domain.User, requestID uuid.UUID, action, note string) (domain.ApprovalRequest, error) {
	if action != "approved" && action != "rejected" {
		return domain.ApprovalRequest{}, ErrValidation
	}
	if !actor.Can("decisions.approve") {
		return domain.ApprovalRequest{}, ErrForbidden
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var item domain.ApprovalRequest
	err = tx.QueryRow(ctx, `SELECT ar.id,ar.decision_id,d.title,ar.requester_id,u.display_name,ar.reviewer_id,ar.state,ar.request_note,ar.response_note,ar.requested_at,ar.resolved_at FROM approval_requests ar JOIN decisions d ON d.id=ar.decision_id JOIN users u ON u.id=ar.requester_id WHERE ar.id=$1 FOR UPDATE`, requestID).Scan(&item.ID, &item.DecisionID, &item.DecisionTitle, &item.RequesterID, &item.RequesterName, &item.ReviewerID, &item.State, &item.RequestNote, &item.ResponseNote, &item.RequestedAt, &item.ResolvedAt)
	if err != nil {
		return item, ErrNotFound
	}
	if item.State != "pending" {
		return item, ErrConflict
	}
	if item.ReviewerID != nil && *item.ReviewerID != actor.ID && !actor.IsAdmin() {
		return item, ErrForbidden
	}
	now := time.Now().UTC()
	item.State = action
	item.ResponseNote = strings.TrimSpace(note)
	item.ResolvedAt = &now
	if _, err := tx.Exec(ctx, `UPDATE approval_requests SET state=$2,response_note=$3,resolved_at=$4 WHERE id=$1`, item.ID, action, item.ResponseNote, now); err != nil {
		return item, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO approval_events(id,approval_request_id,actor_id,action,note) VALUES($1,$2,$3,$4,$5)`, uuid.New(), item.ID, actor.ID, action, item.ResponseNote); err != nil {
		return item, err
	}
	decisionStatus := "draft"
	if action == "approved" {
		decisionStatus = "active"
	}
	if _, err := tx.Exec(ctx, `UPDATE decisions SET workflow_state=$2,status=$3,updated_at=now(),version=version+1 WHERE id=$1`, item.DecisionID, action, decisionStatus); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (s *Store) ListApprovals(ctx context.Context, actor domain.User, state string) ([]domain.ApprovalRequest, error) {
	if state == "" {
		state = "pending"
	}
	rows, err := s.DB.Query(ctx, `SELECT ar.id,ar.decision_id,d.title,ar.requester_id,u.display_name,ar.reviewer_id,ar.state,ar.request_note,ar.response_note,ar.requested_at,ar.resolved_at FROM approval_requests ar JOIN decisions d ON d.id=ar.decision_id JOIN users u ON u.id=ar.requester_id WHERE ar.state=$2 AND ($3 OR ar.reviewer_id=$1 OR (ar.reviewer_id IS NULL AND $4)) ORDER BY ar.requested_at`, actor.ID, state, actor.IsAdmin(), actor.Can("decisions.approve"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ApprovalRequest{}
	for rows.Next() {
		var item domain.ApprovalRequest
		if err := rows.Scan(&item.ID, &item.DecisionID, &item.DecisionTitle, &item.RequesterID, &item.RequesterName, &item.ReviewerID, &item.State, &item.RequestNote, &item.ResponseNote, &item.RequestedAt, &item.ResolvedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
