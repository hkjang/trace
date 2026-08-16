package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

var relationTypes = map[string]bool{
	"DEPENDS_ON": true, "CAUSED_BY": true, "FOLLOW_UP": true, "REPLACES": true,
	"SUPPORTS": true, "CONFLICTS_WITH": true, "RELATED_TO": true,
}

var assumptionStatuses = map[string]bool{
	"UNKNOWN": true, "ACTIVE": true, "STRENGTHENED": true, "WEAKENING": true, "BROKEN": true,
}

func insertDecisionVersion(ctx context.Context, tx pgx.Tx, item domain.Decision, actorID uuid.UUID, reason string, validFrom time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO decision_versions(
		id,decision_id,version,title,category,decision,reason,assumptions,invalidation_conditions,
		confidence,status,workflow_state,decided_at,review_at,change_reason,changed_by,valid_from,created_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		uuid.New(), item.ID, item.Version, item.Title, item.Category, item.Decision, item.Reason,
		item.Assumptions, item.InvalidationConditions, item.Confidence, item.Status, item.WorkflowState,
		item.DecidedAt, item.ReviewAt, reason, actorID, validFrom.UTC(), time.Now().UTC())
	return err
}

func insertConfidenceRecord(ctx context.Context, tx pgx.Tx, actorID, decisionID uuid.UUID, confidence int, reason string, recordedAt time.Time) error {
	if confidence < 0 || confidence > 100 || recordedAt.IsZero() {
		return ErrValidation
	}
	_, err := tx.Exec(ctx, `INSERT INTO decision_confidence_history(id,decision_id,confidence,reason,recorded_by,recorded_at) VALUES($1,$2,$3,$4,$5,$6)`, uuid.New(), decisionID, confidence, strings.TrimSpace(reason), actorID, recordedAt.UTC())
	return err
}

func insertAssumption(ctx context.Context, tx pgx.Tx, actorID, decisionID uuid.UUID, text, status string, knownAt time.Time) (uuid.UUID, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if strings.TrimSpace(text) == "" || !assumptionStatuses[status] || knownAt.IsZero() {
		return uuid.Nil, ErrValidation
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO decision_assumptions(id,decision_id,assumption,status,created_by,known_at) VALUES($1,$2,$3,$4,$5,$6)`, id, decisionID, strings.TrimSpace(text), status, actorID, knownAt.UTC()); err != nil {
		return uuid.Nil, err
	}
	_, err := tx.Exec(ctx, `INSERT INTO assumption_events(id,assumption_id,status,reason,changed_by,known_at) VALUES($1,$2,$3,'Initial assumption',$4,$5)`, uuid.New(), id, status, actorID, knownAt.UTC())
	return id, err
}

func (s *Store) restoreDecisionVersion(ctx context.Context, item *domain.Decision, at time.Time) error {
	var version domain.DecisionVersion
	err := s.DB.QueryRow(ctx, `SELECT id,decision_id,version,title,category,decision,reason,assumptions,invalidation_conditions,confidence,status,workflow_state,decided_at,review_at,change_reason,changed_by,valid_from,valid_to,created_at
		FROM decision_versions WHERE decision_id=$1 AND valid_from<=$2 ORDER BY valid_from DESC,version DESC LIMIT 1`, item.ID, at.UTC()).Scan(
		&version.ID, &version.DecisionID, &version.Version, &version.Title, &version.Category, &version.Decision,
		&version.Reason, &version.Assumptions, &version.InvalidationConditions, &version.Confidence,
		&version.Status, &version.WorkflowState, &version.DecidedAt, &version.ReviewAt, &version.ChangeReason,
		&version.ChangedBy, &version.ValidFrom, &version.ValidTo, &version.CreatedAt)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	item.Title, item.Category, item.Decision, item.Reason = version.Title, version.Category, version.Decision, version.Reason
	item.Assumptions, item.InvalidationConditions = version.Assumptions, version.InvalidationConditions
	item.Confidence, item.Status, item.WorkflowState, item.DecidedAt, item.ReviewAt, item.Version = version.Confidence, version.Status, version.WorkflowState, version.DecidedAt, version.ReviewAt, version.Version
	return nil
}

func (s *Store) loadDecisionIntelligence(ctx context.Context, item *domain.Decision, replayAt *time.Time) error {
	cutoff := time.Now().UTC().Add(100 * 365 * 24 * time.Hour)
	if replayAt != nil {
		cutoff = replayAt.UTC()
	}
	rows, err := s.DB.Query(ctx, `SELECT id,decision_id,confidence,reason,recorded_at,created_at FROM decision_confidence_history WHERE decision_id=$1 AND recorded_at<=$2 ORDER BY recorded_at,created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var value domain.ConfidenceRecord
		if err := rows.Scan(&value.ID, &value.DecisionID, &value.Confidence, &value.Reason, &value.RecordedAt, &value.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		item.ConfidenceHistory = append(item.ConfidenceHistory, value)
	}
	rows.Close()
	if len(item.ConfidenceHistory) > 0 {
		item.Confidence = item.ConfidenceHistory[len(item.ConfidenceHistory)-1].Confidence
	}

	rows, err = s.DB.Query(ctx, `SELECT a.id,a.decision_id,a.assumption,COALESCE(e.status,a.status),a.known_at,a.created_at,a.updated_at
		FROM decision_assumptions a
		LEFT JOIN LATERAL (SELECT status FROM assumption_events ae WHERE ae.assumption_id=a.id AND ae.known_at<=$2 ORDER BY ae.known_at DESC,ae.created_at DESC LIMIT 1) e ON true
		WHERE a.decision_id=$1 AND a.known_at<=$2 ORDER BY a.known_at,a.created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	assumptionIndex := map[uuid.UUID]int{}
	for rows.Next() {
		var value domain.Assumption
		if err := rows.Scan(&value.ID, &value.DecisionID, &value.Assumption, &value.Status, &value.KnownAt, &value.CreatedAt, &value.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		assumptionIndex[value.ID] = len(item.AssumptionItems)
		item.AssumptionItems = append(item.AssumptionItems, value)
	}
	rows.Close()
	if len(assumptionIndex) > 0 {
		rows, err = s.DB.Query(ctx, `SELECT ae.id,ae.assumption_id,ae.previous_status,ae.status,ae.reason,ae.evidence_id,ae.known_at,ae.created_at FROM assumption_events ae JOIN decision_assumptions a ON a.id=ae.assumption_id WHERE a.decision_id=$1 AND ae.known_at<=$2 ORDER BY ae.known_at,ae.created_at`, item.ID, cutoff)
		if err != nil {
			return err
		}
		for rows.Next() {
			var value domain.AssumptionEvent
			var assumptionID uuid.UUID
			if err := rows.Scan(&value.ID, &assumptionID, &value.PreviousStatus, &value.Status, &value.Reason, &value.EvidenceID, &value.KnownAt, &value.CreatedAt); err != nil {
				rows.Close()
				return err
			}
			if index, ok := assumptionIndex[assumptionID]; ok {
				item.AssumptionItems[index].Events = append(item.AssumptionItems[index].Events, value)
			}
		}
		rows.Close()
	}

	rows, err = s.DB.Query(ctx, `SELECT i.id,i.decision_id,i.condition,COALESCE(ev.status,i.status),COALESCE(ev.evidence_id,i.evidence_id),COALESCE(ev.note,i.detection_note),i.known_at,CASE WHEN i.triggered_at<=$2 THEN i.triggered_at END,i.created_at,i.updated_at
		FROM invalidation_conditions i
		LEFT JOIN LATERAL(SELECT status,evidence_id,note FROM invalidation_events x WHERE x.condition_id=i.id AND x.known_at<=$2 ORDER BY x.known_at DESC,x.created_at DESC LIMIT 1) ev ON true
		WHERE i.decision_id=$1 AND i.known_at<=$2 ORDER BY i.known_at,i.created_at`, item.ID, cutoff)
	if err != nil {
		return err
	}
	for rows.Next() {
		var value domain.InvalidationCondition
		if err := rows.Scan(&value.ID, &value.DecisionID, &value.Condition, &value.Status, &value.EvidenceID, &value.DetectionNote, &value.KnownAt, &value.TriggeredAt, &value.CreatedAt, &value.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		item.Invalidations = append(item.Invalidations, value)
	}
	rows.Close()

	var score domain.DecisionScore
	err = s.DB.QueryRow(ctx, `SELECT ds.evidence_quality,ds.logic_quality,ds.alternative_consideration,ds.risk_awareness,ds.assumption_quality,ds.calibration,ds.counter_evidence,ds.overall,ar.replay_at,ds.estimated_at FROM decision_scores ds LEFT JOIN ai_analysis_runs ar ON ar.id=ds.analysis_run_id WHERE ds.decision_id=$1 AND ds.estimated_at<=$2 ORDER BY ds.estimated_at DESC LIMIT 1`, item.ID, cutoff).Scan(
		&score.EvidenceQuality, &score.LogicQuality, &score.AlternativeConsideration, &score.RiskAwareness,
		&score.AssumptionQuality, &score.Calibration, &score.CounterEvidence, &score.Overall, &score.ReplayAt, &score.EstimatedAt)
	if err == nil {
		item.LatestScore = &score
	} else if err != pgx.ErrNoRows {
		return err
	}
	return nil
}

func decisionHealth(item domain.Decision, at time.Time) string {
	atRisk := 0
	broken := false
	for _, assumption := range item.AssumptionItems {
		if assumption.Status == "BROKEN" {
			broken = true
		}
		if assumption.Status == "BROKEN" || assumption.Status == "WEAKENING" {
			atRisk++
		}
	}
	triggered := false
	for _, condition := range item.Invalidations {
		triggered = triggered || condition.Status == "TRIGGERED"
	}
	against := 0
	for _, evidence := range item.Evidence {
		if evidence.Stance == "against" {
			against++
		}
	}
	if broken || triggered {
		return "CRITICAL"
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if atRisk > 0 || (item.ReviewAt != nil && item.ReviewAt.Before(at)) {
		return "NEEDS_REVIEW"
	}
	if against > 0 {
		return "WATCH"
	}
	return "HEALTHY"
}

func (s *Store) ListDecisionVersions(ctx context.Context, actor domain.User, decisionID uuid.UUID) ([]domain.DecisionVersion, error) {
	if _, err := s.GetDecision(ctx, actor, decisionID, nil); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(ctx, `SELECT id,decision_id,version,title,category,decision,reason,assumptions,invalidation_conditions,confidence,status,workflow_state,decided_at,review_at,change_reason,changed_by,valid_from,valid_to,created_at FROM decision_versions WHERE decision_id=$1 ORDER BY version DESC`, decisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.DecisionVersion{}
	for rows.Next() {
		var value domain.DecisionVersion
		if err := rows.Scan(&value.ID, &value.DecisionID, &value.Version, &value.Title, &value.Category, &value.Decision, &value.Reason, &value.Assumptions, &value.InvalidationConditions, &value.Confidence, &value.Status, &value.WorkflowState, &value.DecidedAt, &value.ReviewAt, &value.ChangeReason, &value.ChangedBy, &value.ValidFrom, &value.ValidTo, &value.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) AddConfidence(ctx context.Context, actor domain.User, decisionID uuid.UUID, confidence int, reason string, recordedAt time.Time) (domain.ConfidenceRecord, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.ConfidenceRecord{}, err
	}
	if confidence < 0 || confidence > 100 || strings.TrimSpace(reason) == "" {
		return domain.ConfidenceRecord{}, ErrValidation
	}
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.ConfidenceRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	record := domain.ConfidenceRecord{ID: uuid.New(), DecisionID: decisionID, Confidence: confidence, Reason: strings.TrimSpace(reason), RecordedAt: recordedAt.UTC(), CreatedAt: time.Now().UTC()}
	if _, err := tx.Exec(ctx, `INSERT INTO decision_confidence_history(id,decision_id,confidence,reason,recorded_by,recorded_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, record.ID, decisionID, confidence, record.Reason, actor.ID, record.RecordedAt, record.CreatedAt); err != nil {
		return record, err
	}
	if _, err := tx.Exec(ctx, `UPDATE decisions SET confidence=$2,updated_at=now() WHERE id=$1`, decisionID, confidence); err != nil {
		return record, err
	}
	payload, _ := json.Marshal(map[string]any{"confidence": confidence, "reason": record.Reason})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'confidence.changed',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, record.RecordedAt); err != nil {
		return record, err
	}
	return record, tx.Commit(ctx)
}

func (s *Store) AddAssumption(ctx context.Context, actor domain.User, decisionID uuid.UUID, text, status string, knownAt time.Time) (domain.Assumption, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.Assumption{}, err
	}
	if knownAt.IsZero() {
		knownAt = time.Now().UTC()
	}
	status = strings.ToUpper(defaultString(status, "UNKNOWN"))
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Assumption{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	assumptionID, err := insertAssumption(ctx, tx, actor.ID, decisionID, text, status, knownAt)
	if err != nil {
		return domain.Assumption{}, err
	}
	var value domain.Assumption
	if err := tx.QueryRow(ctx, `SELECT id,decision_id,assumption,status,known_at,created_at,updated_at FROM decision_assumptions WHERE id=$1`, assumptionID).Scan(&value.ID, &value.DecisionID, &value.Assumption, &value.Status, &value.KnownAt, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]any{"assumptionId": value.ID, "status": value.Status})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'assumption.added',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, knownAt.UTC()); err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func (s *Store) UpdateAssumption(ctx context.Context, actor domain.User, assumptionID uuid.UUID, status, reason string, evidenceID *uuid.UUID, knownAt time.Time) (domain.Assumption, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if !assumptionStatuses[status] || strings.TrimSpace(reason) == "" {
		return domain.Assumption{}, ErrValidation
	}
	if knownAt.IsZero() {
		knownAt = time.Now().UTC()
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Assumption{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var value domain.Assumption
	var ownerID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT a.id,a.decision_id,a.assumption,a.status,a.known_at,a.created_at,a.updated_at,d.owner_id FROM decision_assumptions a JOIN decisions d ON d.id=a.decision_id WHERE a.id=$1 FOR UPDATE`, assumptionID).Scan(&value.ID, &value.DecisionID, &value.Assumption, &value.Status, &value.KnownAt, &value.CreatedAt, &value.UpdatedAt, &ownerID); err == pgx.ErrNoRows {
		return value, ErrNotFound
	} else if err != nil {
		return value, err
	}
	if ownerID != actor.ID && !actor.IsAdmin() {
		return value, ErrForbidden
	}
	previous := value.Status
	if _, err := tx.Exec(ctx, `UPDATE decision_assumptions SET status=$2,updated_at=now() WHERE id=$1`, assumptionID, status); err != nil {
		return value, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO assumption_events(id,assumption_id,previous_status,status,reason,evidence_id,changed_by,known_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.New(), assumptionID, previous, status, strings.TrimSpace(reason), evidenceID, actor.ID, knownAt.UTC()); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]any{"assumptionId": assumptionID, "from": previous, "to": status, "reason": strings.TrimSpace(reason)})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'assumption.changed',$4,$5,$5)`, uuid.New(), value.DecisionID, actor.ID, payload, knownAt.UTC()); err != nil {
		return value, err
	}
	value.Status, value.UpdatedAt = status, time.Now().UTC()
	return value, tx.Commit(ctx)
}

func (s *Store) AddInvalidationCondition(ctx context.Context, actor domain.User, decisionID uuid.UUID, condition string, knownAt time.Time) (domain.InvalidationCondition, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.InvalidationCondition{}, err
	}
	if strings.TrimSpace(condition) == "" {
		return domain.InvalidationCondition{}, ErrValidation
	}
	if knownAt.IsZero() {
		knownAt = time.Now().UTC()
	}
	value := domain.InvalidationCondition{ID: uuid.New(), DecisionID: decisionID, Condition: strings.TrimSpace(condition), Status: "ACTIVE", KnownAt: knownAt.UTC(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return value, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO invalidation_conditions(id,decision_id,condition,status,created_by,known_at,created_at,updated_at) VALUES($1,$2,$3,'ACTIVE',$4,$5,$6,$7)`, value.ID, decisionID, value.Condition, actor.ID, value.KnownAt, value.CreatedAt, value.UpdatedAt); err != nil {
		return value, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO invalidation_events(id,condition_id,status,note,changed_by,known_at) VALUES($1,$2,'ACTIVE','Initial invalidation condition',$3,$4)`, uuid.New(), value.ID, actor.ID, value.KnownAt); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]any{"conditionId": value.ID, "condition": value.Condition})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'invalidation.added',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, value.KnownAt); err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func (s *Store) UpdateInvalidationCondition(ctx context.Context, actor domain.User, conditionID uuid.UUID, status, note string, evidenceID *uuid.UUID, knownAt time.Time) (domain.InvalidationCondition, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "ACTIVE" && status != "TRIGGERED" && status != "RESOLVED" {
		return domain.InvalidationCondition{}, ErrValidation
	}
	if status != "ACTIVE" && strings.TrimSpace(note) == "" {
		return domain.InvalidationCondition{}, ErrValidation
	}
	if knownAt.IsZero() {
		knownAt = time.Now().UTC()
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.InvalidationCondition{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var value domain.InvalidationCondition
	var ownerID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT i.id,i.decision_id,i.condition,i.status,i.evidence_id,i.detection_note,i.known_at,i.triggered_at,i.created_at,i.updated_at,d.owner_id FROM invalidation_conditions i JOIN decisions d ON d.id=i.decision_id WHERE i.id=$1 FOR UPDATE`, conditionID).Scan(&value.ID, &value.DecisionID, &value.Condition, &value.Status, &value.EvidenceID, &value.DetectionNote, &value.KnownAt, &value.TriggeredAt, &value.CreatedAt, &value.UpdatedAt, &ownerID); err == pgx.ErrNoRows {
		return value, ErrNotFound
	} else if err != nil {
		return value, err
	}
	if ownerID != actor.ID && !actor.IsAdmin() {
		return value, ErrForbidden
	}
	var triggeredAt *time.Time
	if status == "TRIGGERED" {
		triggered := knownAt.UTC()
		triggeredAt = &triggered
	}
	if status == "RESOLVED" {
		triggeredAt = value.TriggeredAt
	}
	if _, err := tx.Exec(ctx, `UPDATE invalidation_conditions SET status=$2,evidence_id=COALESCE($3,evidence_id),detection_note=$4,triggered_at=$5,updated_at=now() WHERE id=$1`, conditionID, status, evidenceID, strings.TrimSpace(note), triggeredAt); err != nil {
		return value, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO invalidation_events(id,condition_id,previous_status,status,note,evidence_id,changed_by,known_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.New(), conditionID, value.Status, status, strings.TrimSpace(note), evidenceID, actor.ID, knownAt.UTC()); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]any{"conditionId": conditionID, "from": value.Status, "to": status, "note": strings.TrimSpace(note), "evidenceId": evidenceID})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'invalidation.changed',$4,$5,$5)`, uuid.New(), value.DecisionID, actor.ID, payload, knownAt.UTC()); err != nil {
		return value, err
	}
	value.Status, value.EvidenceID, value.DetectionNote, value.TriggeredAt, value.UpdatedAt = status, evidenceID, strings.TrimSpace(note), triggeredAt, time.Now().UTC()
	return value, tx.Commit(ctx)
}

func (s *Store) AddDecisionLink(ctx context.Context, actor domain.User, sourceID, targetID uuid.UUID, relation, description string, effectiveAt time.Time) (domain.DecisionLink, error) {
	if sourceID == targetID {
		return domain.DecisionLink{}, ErrValidation
	}
	relation = strings.ToUpper(strings.TrimSpace(relation))
	if !relationTypes[relation] {
		return domain.DecisionLink{}, ErrValidation
	}
	if err := s.ensureDecisionEditor(ctx, actor, sourceID); err != nil {
		return domain.DecisionLink{}, err
	}
	if _, err := s.GetDecision(ctx, actor, targetID, nil); err != nil {
		return domain.DecisionLink{}, err
	}
	if effectiveAt.IsZero() {
		effectiveAt = time.Now().UTC()
	}
	value := domain.DecisionLink{ID: uuid.New(), SourceDecisionID: sourceID, TargetDecisionID: targetID, RelationType: relation, Description: strings.TrimSpace(description), EffectiveAt: effectiveAt.UTC(), CreatedAt: time.Now().UTC()}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return value, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO decision_links(id,source_decision_id,target_decision_id,relation_type,description,effective_at,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, value.ID, sourceID, targetID, relation, value.Description, value.EffectiveAt, actor.ID, value.CreatedAt); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]any{"linkId": value.ID, "targetDecisionId": targetID, "relationType": relation})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'decision.linked',$4,$5,$5)`, uuid.New(), sourceID, actor.ID, payload, value.EffectiveAt); err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func (s *Store) DeleteDecisionLink(ctx context.Context, actor domain.User, linkID uuid.UUID) error {
	var sourceID, targetID uuid.UUID
	var relation string
	if err := s.DB.QueryRow(ctx, `SELECT source_decision_id,target_decision_id,relation_type FROM decision_links WHERE id=$1 AND deleted_at IS NULL`, linkID).Scan(&sourceID, &targetID, &relation); err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.ensureDecisionEditor(ctx, actor, sourceID); err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	deletedAt := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE decision_links SET deleted_at=$2 WHERE id=$1`, linkID, deletedAt); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"linkId": linkID, "targetDecisionId": targetID, "relationType": relation})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'decision.unlinked',$4,$5,$5)`, uuid.New(), sourceID, actor.ID, payload, deletedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DecisionGraph(ctx context.Context, actor domain.User, focusID *uuid.UUID, depth int, category string, at *time.Time, limit int) (domain.DecisionGraph, error) {
	if depth < 1 || depth > 2 {
		depth = 1
	}
	if limit < 1 || limit > 500 {
		limit = 200
	}
	cutoff := time.Now().UTC().Add(100 * 365 * 24 * time.Hour)
	if at != nil {
		cutoff = at.UTC()
	}
	rows, err := s.DB.Query(ctx, `SELECT d.id,COALESCE(v.title,d.title),COALESCE(v.category,d.category),COALESCE(v.status,d.status),COALESCE(ch.confidence,v.confidence,d.confidence),o.outcome_score,COALESCE(v.decided_at,d.decided_at),
		CASE
			WHEN EXISTS(SELECT 1 FROM decision_assumptions a JOIN LATERAL(SELECT status FROM assumption_events ae WHERE ae.assumption_id=a.id AND ae.known_at<=$2 ORDER BY ae.known_at DESC,ae.created_at DESC LIMIT 1) x ON true WHERE a.decision_id=d.id AND a.known_at<=$2 AND x.status='BROKEN')
			  OR EXISTS(SELECT 1 FROM invalidation_conditions i JOIN LATERAL(SELECT status FROM invalidation_events ie WHERE ie.condition_id=i.id AND ie.known_at<=$2 ORDER BY ie.known_at DESC,ie.created_at DESC LIMIT 1) x ON true WHERE i.decision_id=d.id AND i.known_at<=$2 AND x.status='TRIGGERED') THEN 'CRITICAL'
			WHEN EXISTS(SELECT 1 FROM decision_assumptions a JOIN LATERAL(SELECT status FROM assumption_events ae WHERE ae.assumption_id=a.id AND ae.known_at<=$2 ORDER BY ae.known_at DESC,ae.created_at DESC LIMIT 1) x ON true WHERE a.decision_id=d.id AND a.known_at<=$2 AND x.status='WEAKENING')
			  OR COALESCE(v.review_at,d.review_at)<=$2 THEN 'NEEDS_REVIEW'
			ELSE 'HEALTHY'
		END
		FROM decisions d
		LEFT JOIN LATERAL(SELECT * FROM decision_versions dv WHERE dv.decision_id=d.id AND dv.valid_from<=$2 ORDER BY dv.valid_from DESC,dv.version DESC LIMIT 1) v ON true
		LEFT JOIN LATERAL(SELECT confidence FROM decision_confidence_history h WHERE h.decision_id=d.id AND h.recorded_at<=$2 ORDER BY h.recorded_at DESC LIMIT 1) ch ON true
		LEFT JOIN LATERAL(SELECT outcome_score FROM decision_outcomes x WHERE x.decision_id=d.id AND x.outcome_at<=$2 ORDER BY x.outcome_at DESC LIMIT 1) o ON true
		WHERE d.decided_at<=$2 AND ($3='' OR COALESCE(v.category,d.category)=$3) AND ($4 OR d.owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))
		ORDER BY d.updated_at DESC LIMIT $5`, actor.ID, cutoff, strings.TrimSpace(category), actor.IsAdmin(), limit)
	if err != nil {
		return domain.DecisionGraph{}, err
	}
	defer rows.Close()
	nodes := map[uuid.UUID]domain.GraphNode{}
	for rows.Next() {
		var node domain.GraphNode
		if err := rows.Scan(&node.ID, &node.Title, &node.Category, &node.Status, &node.Confidence, &node.Outcome, &node.DecidedAt, &node.Health); err != nil {
			return domain.DecisionGraph{}, err
		}
		nodes[node.ID] = node
	}
	if err := rows.Err(); err != nil {
		return domain.DecisionGraph{}, err
	}
	if focusID != nil {
		if _, ok := nodes[*focusID]; !ok {
			return domain.DecisionGraph{}, ErrNotFound
		}
	}
	edgeRows, err := s.DB.Query(ctx, `SELECT id,source_decision_id,target_decision_id,relation_type,description,effective_at,created_at FROM decision_links WHERE effective_at<=$1 AND (deleted_at IS NULL OR deleted_at>$1) ORDER BY created_at`, cutoff)
	if err != nil {
		return domain.DecisionGraph{}, err
	}
	defer edgeRows.Close()
	edges := []domain.DecisionLink{}
	for edgeRows.Next() {
		var edge domain.DecisionLink
		if err := edgeRows.Scan(&edge.ID, &edge.SourceDecisionID, &edge.TargetDecisionID, &edge.RelationType, &edge.Description, &edge.EffectiveAt, &edge.CreatedAt); err != nil {
			return domain.DecisionGraph{}, err
		}
		if _, sourceOK := nodes[edge.SourceDecisionID]; sourceOK {
			if _, targetOK := nodes[edge.TargetDecisionID]; targetOK {
				edges = append(edges, edge)
			}
		}
	}
	selected := map[uuid.UUID]int{}
	if focusID == nil {
		for id := range nodes {
			selected[id] = 0
		}
	} else {
		selected[*focusID] = 0
		frontier := []uuid.UUID{*focusID}
		for hop := 1; hop <= depth; hop++ {
			next := []uuid.UUID{}
			for _, current := range frontier {
				for _, edge := range edges {
					neighbor := uuid.Nil
					if edge.SourceDecisionID == current {
						neighbor = edge.TargetDecisionID
					} else if edge.TargetDecisionID == current {
						neighbor = edge.SourceDecisionID
					}
					if neighbor != uuid.Nil {
						if _, exists := selected[neighbor]; !exists {
							selected[neighbor] = hop
							next = append(next, neighbor)
						}
					}
				}
			}
			frontier = next
		}
	}
	result := domain.DecisionGraph{At: at, Nodes: []domain.GraphNode{}, Edges: []domain.DecisionLink{}}
	if focusID != nil {
		result.FocusID = *focusID
	}
	for id, node := range nodes {
		if nodeDepth, ok := selected[id]; ok {
			node.Depth = nodeDepth
			result.Nodes = append(result.Nodes, node)
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool {
		if result.Nodes[i].Depth == result.Nodes[j].Depth {
			return result.Nodes[i].DecidedAt.Before(result.Nodes[j].DecidedAt)
		}
		return result.Nodes[i].Depth < result.Nodes[j].Depth
	})
	for _, edge := range edges {
		_, sourceOK := selected[edge.SourceDecisionID]
		_, targetOK := selected[edge.TargetDecisionID]
		if sourceOK && targetOK {
			result.Edges = append(result.Edges, edge)
		}
	}
	return result, nil
}

func replaySnapshot(at time.Time, decision domain.Decision) domain.ReplaySnapshot {
	result := domain.ReplaySnapshot{At: at, Version: decision.Version, Confidence: decision.Confidence, EvidenceCount: len(decision.Evidence), AlternativeCount: len(decision.Alternatives), AssumptionCount: len(decision.AssumptionItems), OutcomeCount: len(decision.Outcomes), Decision: decision}
	for _, assumption := range decision.AssumptionItems {
		if assumption.Status == "WEAKENING" || assumption.Status == "BROKEN" {
			result.AtRiskAssumptions++
		}
	}
	if len(decision.Outcomes) > 0 {
		value := decision.Outcomes[len(decision.Outcomes)-1].OutcomeScore
		result.LatestOutcomeScore = &value
	}
	return result
}

func (s *Store) CompareReplay(ctx context.Context, actor domain.User, decisionID uuid.UUID, from, to time.Time) (domain.ReplayComparison, error) {
	if from.After(to) {
		from, to = to, from
	}
	before, err := s.GetDecision(ctx, actor, decisionID, &from)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	after, err := s.GetDecision(ctx, actor, decisionID, &to)
	if err != nil {
		return domain.ReplayComparison{}, err
	}
	result := domain.ReplayComparison{From: replaySnapshot(from, before), To: replaySnapshot(to, after), Changes: []domain.ReplayChange{}}
	appendChange := func(kind, label, oldValue, newValue, description string) {
		if oldValue != newValue {
			result.Changes = append(result.Changes, domain.ReplayChange{Kind: kind, Label: label, Before: oldValue, After: newValue, Description: description})
		}
	}
	appendChange("version", "Decision version", fmt.Sprint(before.Version), fmt.Sprint(after.Version), "결정 본문의 시점별 버전")
	appendChange("decision", "Decision", before.Decision, after.Decision, "선택 범위 또는 결론이 바뀌었습니다.")
	appendChange("confidence", "Confidence", fmt.Sprintf("%d%%", before.Confidence), fmt.Sprintf("%d%%", after.Confidence), "확신 수준의 변화")
	appendChange("evidence", "Evidence", fmt.Sprint(len(before.Evidence)), fmt.Sprint(len(after.Evidence)), "새롭게 알게 된 근거")
	appendChange("assumption", "At-risk assumptions", fmt.Sprint(result.From.AtRiskAssumptions), fmt.Sprint(result.To.AtRiskAssumptions), "약화되거나 깨진 전제")
	appendChange("outcome", "Outcomes", fmt.Sprint(len(before.Outcomes)), fmt.Sprint(len(after.Outcomes)), "확인된 실제 결과")
	return result, nil
}

type MemoryDocument struct {
	Decision       domain.Decision
	Text           string
	Excerpt        string
	LastOutcome    *int
	Reliability    float64
	RelatedToFocus bool
}

func (s *Store) MemoryDocuments(ctx context.Context, actor domain.User, limit int) ([]MemoryDocument, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	items := []domain.Decision{}
	for offset := 0; len(items) < limit; offset += 100 {
		batchSize := limit - len(items)
		if batchSize > 100 {
			batchSize = 100
		}
		batch, err := s.ListDecisions(ctx, actor, "", "", batchSize, offset)
		if err != nil {
			return nil, err
		}
		items = append(items, batch...)
		if len(batch) < batchSize {
			break
		}
	}
	result := make([]MemoryDocument, 0, len(items))
	for _, summary := range items {
		item, err := s.GetDecision(ctx, actor, summary.ID, nil)
		if err != nil {
			return nil, err
		}
		parts := []string{item.Title, item.Category, item.Decision, item.Reason, item.Assumptions, item.InvalidationConditions}
		reliabilitySum, reliabilityCount := 0, 0
		for _, evidence := range item.Evidence {
			parts = append(parts, evidence.Title, evidence.Content, evidence.Summary)
			if evidence.Reliability != nil {
				reliabilitySum += *evidence.Reliability
				reliabilityCount++
			}
		}
		for _, reflection := range item.Reflections {
			parts = append(parts, reflection.Reflection, reflection.Learning)
		}
		var outcome *int
		if len(item.Outcomes) > 0 {
			value := item.Outcomes[len(item.Outcomes)-1].OutcomeScore
			outcome = &value
		}
		reliability := 0.5
		if reliabilityCount > 0 {
			reliability = float64(reliabilitySum) / float64(reliabilityCount) / 100
		}
		text := strings.Join(parts, "\n")
		excerpt := strings.TrimSpace(item.Decision + " · " + item.Reason)
		if len([]rune(excerpt)) > 180 {
			excerpt = string([]rune(excerpt)[:180]) + "…"
		}
		result = append(result, MemoryDocument{Decision: item, Text: text, Excerpt: excerpt, LastOutcome: outcome, Reliability: reliability})
	}
	return result, nil
}

func LocalEmbedding(text string, dimensions int) []float64 {
	if dimensions < 16 {
		dimensions = 96
	}
	vector := make([]float64, dimensions)
	normalized := strings.ToLower(strings.TrimSpace(text))
	features := strings.FieldsFunc(normalized, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
	runes := []rune(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, normalized))
	for size := 2; size <= 4; size++ {
		for index := 0; index+size <= len(runes); index++ {
			features = append(features, string(runes[index:index+size]))
		}
	}
	for _, feature := range features {
		if feature == "" {
			continue
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(feature))
		value := h.Sum64()
		index := int(value % uint64(dimensions))
		sign := 1.0
		if value&(1<<63) != 0 {
			sign = -1
		}
		vector[index] += sign
	}
	var norm float64
	for _, value := range vector {
		norm += value * value
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for index := range vector {
			vector[index] /= norm
		}
	}
	return vector
}

func CosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func (s *Store) SaveDecisionEmbedding(ctx context.Context, decisionID uuid.UUID, model, text string, embedding []float64) error {
	hash := sha256.Sum256([]byte(text))
	_, err := s.DB.Exec(ctx, `INSERT INTO decision_embeddings(decision_id,model,dimensions,embedding,input_hash,generated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(decision_id) DO UPDATE SET model=excluded.model,dimensions=excluded.dimensions,embedding=excluded.embedding,input_hash=excluded.input_hash,generated_at=now()`, decisionID, model, len(embedding), embedding, hex.EncodeToString(hash[:]))
	return err
}

func (s *Store) StartAnalysisRun(ctx context.Context, userID uuid.UUID, decisionID *uuid.UUID, analysisType, model, promptVersion, contextVersion, inputHash string, replayAt *time.Time) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.DB.Exec(ctx, `INSERT INTO ai_analysis_runs(id,user_id,decision_id,analysis_type,model,prompt_version,context_version,replay_at,input_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, userID, decisionID, analysisType, model, promptVersion, contextVersion, replayAt, inputHash)
	return id, err
}

func (s *Store) CompleteAnalysisRun(ctx context.Context, id uuid.UUID, output any) error {
	raw, err := json.Marshal(output)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `UPDATE ai_analysis_runs SET status='COMPLETED',output_json=$2,completed_at=now() WHERE id=$1`, id, raw)
	return err
}

func (s *Store) FailAnalysisRun(ctx context.Context, id uuid.UUID, message string) error {
	_, err := s.DB.Exec(ctx, `UPDATE ai_analysis_runs SET status='FAILED',error_message=$2,completed_at=now() WHERE id=$1`, id, message)
	return err
}
