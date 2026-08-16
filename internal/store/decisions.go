package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

func validateDecisionInput(input domain.DecisionInput) error {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Decision) == "" {
		return fmt.Errorf("%w: title and decision are required", ErrValidation)
	}
	if input.Confidence < 0 || input.Confidence > 100 {
		return fmt.Errorf("%w: confidence must be between 0 and 100", ErrValidation)
	}
	if input.DecidedAt.IsZero() {
		return fmt.Errorf("%w: decidedAt is required", ErrValidation)
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "draft" {
		return fmt.Errorf("%w: new decision status must be active or draft", ErrValidation)
	}
	return nil
}

func (s *Store) ensureDecisionEditor(ctx context.Context, actor domain.User, decisionID uuid.UUID) error {
	var ownerID uuid.UUID
	if err := s.DB.QueryRow(ctx, `SELECT owner_id FROM decisions WHERE id=$1`, decisionID).Scan(&ownerID); err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if ownerID != actor.ID && !actor.IsAdmin() {
		return ErrForbidden
	}
	return nil
}

func (s *Store) UpdateDecision(ctx context.Context, actor domain.User, decisionID uuid.UUID, patch domain.DecisionPatch) (domain.Decision, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.Decision{}, err
	}
	if patch.Version < 1 {
		return domain.Decision{}, ErrValidation
	}
	if patch.Confidence != nil && (*patch.Confidence < 0 || *patch.Confidence > 100) {
		return domain.Decision{}, ErrValidation
	}
	if patch.Status != nil && *patch.Status != "draft" && *patch.Status != "active" && *patch.Status != "closed" && *patch.Status != "archived" {
		return domain.Decision{}, ErrValidation
	}
	tag, err := s.DB.Exec(ctx, `UPDATE decisions SET title=COALESCE($3,title),category=COALESCE($4,category),decision=COALESCE($5,decision),reason=COALESCE($6,reason),assumptions=COALESCE($7,assumptions),invalidation_conditions=COALESCE($8,invalidation_conditions),confidence=COALESCE($9,confidence),status=COALESCE($10,status),review_at=COALESCE($11,review_at),updated_at=now(),version=version+1 WHERE id=$1 AND version=$2`, decisionID, patch.Version, patch.Title, patch.Category, patch.Decision, patch.Reason, patch.Assumptions, patch.InvalidationConditions, patch.Confidence, patch.Status, patch.ReviewAt)
	if err != nil {
		return domain.Decision{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Decision{}, ErrConflict
	}
	return s.GetDecision(ctx, actor, decisionID, nil)
}

func (s *Store) CreateDecision(ctx context.Context, actor domain.User, input domain.DecisionInput) (domain.Decision, error) {
	if err := validateDecisionInput(input); err != nil {
		return domain.Decision{}, err
	}
	workflow, err := s.GetWorkflowSettings(ctx)
	if err != nil {
		return domain.Decision{}, err
	}
	status, workflowState := input.Status, "not_required"
	if status == "" {
		status = "active"
	}
	if workflow.ApprovalRequired {
		status, workflowState = "draft", "draft"
	}
	item := domain.Decision{
		ID: uuid.New(), OwnerID: actor.ID, TeamID: input.TeamID,
		Title: strings.TrimSpace(input.Title), Category: defaultString(input.Category, "other"),
		Decision: strings.TrimSpace(input.Decision), Reason: strings.TrimSpace(input.Reason),
		Assumptions: strings.TrimSpace(input.Assumptions), InvalidationConditions: strings.TrimSpace(input.InvalidationConditions),
		Confidence: input.Confidence, Status: status, WorkflowState: workflowState,
		DecidedAt: input.DecidedAt.UTC(), ReviewAt: input.ReviewAt, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Version: 1,
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Decision{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if input.TeamID != nil {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id=$1 AND user_id=$2)`, input.TeamID, actor.ID).Scan(&allowed); err != nil || (!allowed && !actor.IsAdmin()) {
			return domain.Decision{}, ErrForbidden
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO decisions(id,owner_id,team_id,title,category,decision,reason,assumptions,invalidation_conditions,confidence,status,workflow_state,decided_at,review_at,created_at,updated_at,version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1)`,
		item.ID, item.OwnerID, item.TeamID, item.Title, item.Category, item.Decision, item.Reason, item.Assumptions, item.InvalidationConditions, item.Confidence, item.Status, item.WorkflowState, item.DecidedAt, item.ReviewAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("insert decision: %w", err)
	}
	for _, alternative := range input.Alternatives {
		if strings.TrimSpace(alternative.Title) == "" {
			continue
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO decision_alternatives(id,decision_id,title,description) VALUES($1,$2,$3,$4)`, id, item.ID, strings.TrimSpace(alternative.Title), strings.TrimSpace(alternative.Description)); err != nil {
			return domain.Decision{}, err
		}
		item.Alternatives = append(item.Alternatives, domain.Alternative{ID: id, Title: alternative.Title, Description: alternative.Description})
	}
	for _, evidence := range input.Evidence {
		added, err := insertEvidence(ctx, tx, actor.ID, item.ID, evidence)
		if err != nil {
			return domain.Decision{}, err
		}
		item.Evidence = append(item.Evidence, added)
	}
	if input.Expectation != nil && strings.TrimSpace(input.Expectation.Expectation) != "" {
		expectation := domain.Expectation{ID: uuid.New(), Expectation: strings.TrimSpace(input.Expectation.Expectation), SuccessCriteria: strings.TrimSpace(input.Expectation.SuccessCriteria), ExpectedAt: input.Expectation.ExpectedAt, Probability: input.Expectation.Probability}
		if _, err := tx.Exec(ctx, `INSERT INTO decision_expectations(id,decision_id,expectation,success_criteria,expected_at,probability) VALUES($1,$2,$3,$4,$5,$6)`, expectation.ID, item.ID, expectation.Expectation, expectation.SuccessCriteria, expectation.ExpectedAt, expectation.Probability); err != nil {
			return domain.Decision{}, err
		}
		item.Expectations = []domain.Expectation{expectation}
	}
	payload, _ := json.Marshal(map[string]any{"title": item.Title, "confidence": item.Confidence})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'decision.created',$4,$5,$5)`, uuid.New(), item.ID, actor.ID, payload, item.DecidedAt); err != nil {
		return domain.Decision{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_id,action,resource_type,resource_id) VALUES($1,$2,'decision.create','decision',$3)`, uuid.New(), actor.ID, item.ID.String()); err != nil {
		return domain.Decision{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Decision{}, err
	}
	return item, nil
}

func insertEvidence(ctx context.Context, tx pgx.Tx, actorID, decisionID uuid.UUID, input domain.EvidenceInput) (domain.Evidence, error) {
	if strings.TrimSpace(input.Title) == "" || input.KnownAt.IsZero() {
		return domain.Evidence{}, fmt.Errorf("%w: evidence title and knownAt are required", ErrValidation)
	}
	item := domain.Evidence{
		ID: uuid.New(), Title: strings.TrimSpace(input.Title), Type: defaultString(input.Type, "note"),
		Source: strings.TrimSpace(input.Source), Content: input.Content, Snapshot: input.Snapshot,
		Reliability: input.Reliability, Stance: defaultString(input.Stance, "neutral"), PublishedAt: input.PublishedAt,
		KnownAt: input.KnownAt.UTC(), CapturedAt: time.Now().UTC(),
	}
	_, err := tx.Exec(ctx, `INSERT INTO decision_evidence(id,decision_id,title,evidence_type,source,content,snapshot,reliability,stance,published_at,known_at,captured_at,added_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, item.ID, decisionID, item.Title, item.Type, item.Source, item.Content, item.Snapshot, item.Reliability, item.Stance, item.PublishedAt, item.KnownAt, item.CapturedAt, actorID)
	return item, err
}

func (s *Store) ListDecisions(ctx context.Context, actor domain.User, status, query string, limit, offset int) ([]domain.Decision, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.DB.Query(ctx, `SELECT d.id,d.owner_id,d.team_id,d.title,d.category,d.decision,d.reason,d.assumptions,d.invalidation_conditions,d.confidence,d.status,d.workflow_state,d.decided_at,d.review_at,d.created_at,d.updated_at,d.version,u.display_name
		FROM decisions d JOIN users u ON u.id=d.owner_id
		WHERE ($2='' OR d.status=$2) AND ($3='%%' OR lower(d.title) LIKE $3 OR lower(d.decision) LIKE $3)
		AND ($4 OR d.owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))
		ORDER BY d.updated_at DESC LIMIT $5 OFFSET $6`, actor.ID, status, pattern, actor.IsAdmin(), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Decision{}
	for rows.Next() {
		item, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) Dashboard(ctx context.Context, actor domain.User) (domain.Dashboard, error) {
	var result domain.Dashboard
	err := s.DB.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE status='active'),
		count(*) FILTER (WHERE status='active' AND workflow_state='pending'),
		count(*) FILTER (WHERE status='active' AND review_at<=now()),
		count(*) FILTER (WHERE status='closed')
		FROM decisions d WHERE ($2 OR owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))`, actor.ID, actor.IsAdmin()).Scan(&result.ActiveCount, &result.WaitingCount, &result.ReviewDue, &result.ClosedCount)
	if err != nil {
		return result, err
	}
	result.Recent, err = s.ListDecisions(ctx, actor, "", "", 8, 0)
	return result, err
}

func (s *Store) GetDecision(ctx context.Context, actor domain.User, id uuid.UUID, replayAt *time.Time) (domain.Decision, error) {
	row := s.DB.QueryRow(ctx, `SELECT d.id,d.owner_id,d.team_id,d.title,d.category,d.decision,d.reason,d.assumptions,d.invalidation_conditions,d.confidence,d.status,d.workflow_state,d.decided_at,d.review_at,d.created_at,d.updated_at,d.version,u.display_name
		FROM decisions d JOIN users u ON u.id=d.owner_id WHERE d.id=$2 AND ($3 OR d.owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))`, actor.ID, id, actor.IsAdmin())
	item, err := scanDecision(row)
	if err == pgx.ErrNoRows {
		return domain.Decision{}, ErrNotFound
	}
	if err != nil {
		return domain.Decision{}, err
	}
	if replayAt != nil && item.DecidedAt.After(*replayAt) {
		return domain.Decision{}, ErrNotFound
	}
	if err := s.loadDecisionChildren(ctx, &item, replayAt); err != nil {
		return domain.Decision{}, err
	}
	return item, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanDecision(row rowScanner) (domain.Decision, error) {
	var item domain.Decision
	err := row.Scan(&item.ID, &item.OwnerID, &item.TeamID, &item.Title, &item.Category, &item.Decision, &item.Reason, &item.Assumptions, &item.InvalidationConditions, &item.Confidence, &item.Status, &item.WorkflowState, &item.DecidedAt, &item.ReviewAt, &item.CreatedAt, &item.UpdatedAt, &item.Version, &item.OwnerName)
	return item, err
}

func (s *Store) loadDecisionChildren(ctx context.Context, item *domain.Decision, replayAt *time.Time) error {
	cutoff := time.Now().UTC().Add(100 * 365 * 24 * time.Hour)
	if replayAt != nil {
		cutoff = replayAt.UTC()
	}
	rows, err := s.DB.Query(ctx, `SELECT id,title,description FROM decision_alternatives WHERE decision_id=$1 AND created_at<=$2 ORDER BY created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Alternative
		if err := rows.Scan(&v.ID, &v.Title, &v.Description); err != nil {
			rows.Close()
			return err
		}
		item.Alternatives = append(item.Alternatives, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,title,evidence_type,source,content,snapshot,summary,reliability,stance,published_at,known_at,captured_at FROM decision_evidence WHERE decision_id=$1 AND known_at<=$2 ORDER BY known_at,captured_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Evidence
		if err := rows.Scan(&v.ID, &v.Title, &v.Type, &v.Source, &v.Content, &v.Snapshot, &v.Summary, &v.Reliability, &v.Stance, &v.PublishedAt, &v.KnownAt, &v.CapturedAt); err != nil {
			rows.Close()
			return err
		}
		item.Evidence = append(item.Evidence, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,expectation,success_criteria,expected_at,probability FROM decision_expectations WHERE decision_id=$1 AND created_at<=$2 ORDER BY created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Expectation
		if err := rows.Scan(&v.ID, &v.Expectation, &v.SuccessCriteria, &v.ExpectedAt, &v.Probability); err != nil {
			rows.Close()
			return err
		}
		item.Expectations = append(item.Expectations, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,result,outcome_score,decision_quality,outcome_at,created_at FROM decision_outcomes WHERE decision_id=$1 AND outcome_at<=$2 ORDER BY outcome_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Outcome
		if err := rows.Scan(&v.ID, &v.Result, &v.OutcomeScore, &v.DecisionQuality, &v.OutcomeAt, &v.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		item.Outcomes = append(item.Outcomes, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,reflection,learning,reasoning_still_sound,created_at FROM decision_reflections WHERE decision_id=$1 AND created_at<=$2 ORDER BY created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.Reflection
		if err := rows.Scan(&v.ID, &v.Reflection, &v.Learning, &v.ReasoningStillSound, &v.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		item.Reflections = append(item.Reflections, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,insight_type,content,model,prompt_version,replay_at,input_snapshot_hash,generated_at FROM decision_ai_insights WHERE decision_id=$1 AND generated_at<=$2 ORDER BY generated_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.AIInsight
		if err := rows.Scan(&v.ID, &v.InsightType, &v.Content, &v.Model, &v.PromptVersion, &v.ReplayAt, &v.InputSnapshotHash, &v.GeneratedAt); err != nil {
			rows.Close()
			return err
		}
		item.Insights = append(item.Insights, v)
	}
	rows.Close()

	rows, err = s.DB.Query(ctx, `SELECT id,event_type,payload,effective_at,known_at,created_at FROM decision_events WHERE decision_id=$1 AND known_at<=$2 ORDER BY effective_at,created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v domain.DecisionEvent
		if err := rows.Scan(&v.ID, &v.EventType, &v.Payload, &v.EffectiveAt, &v.KnownAt, &v.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		item.Events = append(item.Events, v)
	}
	rows.Close()
	return nil
}

func (s *Store) AddEvidence(ctx context.Context, actor domain.User, decisionID uuid.UUID, input domain.EvidenceInput) (domain.Evidence, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.Evidence{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Evidence{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := insertEvidence(ctx, tx, actor.ID, decisionID, input)
	if err != nil {
		return item, err
	}
	payload, _ := json.Marshal(map[string]any{"evidenceId": item.ID, "title": item.Title, "stance": item.Stance})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'evidence.added',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, item.KnownAt); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (s *Store) AddOutcome(ctx context.Context, actor domain.User, decisionID uuid.UUID, input domain.Outcome) (domain.Outcome, error) {
	if strings.TrimSpace(input.Result) == "" || input.OutcomeScore < -2 || input.OutcomeScore > 2 || input.OutcomeAt.IsZero() {
		return domain.Outcome{}, ErrValidation
	}
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.Outcome{}, err
	}
	input.ID = uuid.New()
	input.CreatedAt = time.Now().UTC()
	input.OutcomeAt = input.OutcomeAt.UTC()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Outcome{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO decision_outcomes(id,decision_id,result,outcome_score,decision_quality,outcome_at,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, input.ID, decisionID, input.Result, input.OutcomeScore, input.DecisionQuality, input.OutcomeAt, actor.ID, input.CreatedAt); err != nil {
		return domain.Outcome{}, err
	}
	payload, _ := json.Marshal(map[string]any{"outcomeId": input.ID, "score": input.OutcomeScore})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'outcome.recorded',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, input.OutcomeAt); err != nil {
		return domain.Outcome{}, err
	}
	return input, tx.Commit(ctx)
}

func (s *Store) AddReflection(ctx context.Context, actor domain.User, decisionID uuid.UUID, input domain.Reflection) (domain.Reflection, error) {
	if strings.TrimSpace(input.Reflection) == "" {
		return domain.Reflection{}, ErrValidation
	}
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.Reflection{}, err
	}
	input.ID = uuid.New()
	input.CreatedAt = time.Now().UTC()
	_, err := s.DB.Exec(ctx, `INSERT INTO decision_reflections(id,decision_id,reflection,learning,reasoning_still_sound,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, input.ID, decisionID, input.Reflection, input.Learning, input.ReasoningStillSound, actor.ID, input.CreatedAt)
	return input, err
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
