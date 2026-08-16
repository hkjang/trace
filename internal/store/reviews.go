package store

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/jackc/pgx/v5"
)

type reviewSignals struct {
	OutcomeDue        bool
	HighConfidence    bool
	NewEvidence       bool
	AssumptionAtRisk  bool
	LongTimeNoReview  bool
	InvalidationFired bool
}

func reviewPriority(signals reviewSignals) (int, []string) {
	score := 0
	reasons := []string{}
	if signals.OutcomeDue {
		score += 30
		reasons = append(reasons, "예상 결과 또는 정기 검토일 도래")
	}
	if signals.HighConfidence {
		score += 20
		reasons = append(reasons, "높은 확신으로 기록한 판단")
	}
	if signals.NewEvidence {
		score += 20
		reasons = append(reasons, "마지막 검토 이후 새 근거")
	}
	if signals.AssumptionAtRisk {
		score += 20
		reasons = append(reasons, "약화되거나 깨진 전제")
	}
	if signals.LongTimeNoReview {
		score += 10
		reasons = append(reasons, "90일 이상 검토하지 않음")
	}
	if signals.InvalidationFired {
		score += 20
		reasons = append(reasons, "반증 조건이 감지됨")
	}
	if score > 100 {
		score = 100
	}
	return score, reasons
}

func reviewHealth(score int, signals reviewSignals) string {
	if signals.InvalidationFired || score >= 80 {
		return "CRITICAL"
	}
	if signals.AssumptionAtRisk || score >= 40 {
		return "NEEDS_REVIEW"
	}
	if score >= 20 {
		return "WATCH"
	}
	return "HEALTHY"
}

func (s *Store) ReviewQueue(ctx context.Context, actor domain.User, limit int) ([]domain.ReviewItem, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.DB.Query(ctx, `SELECT d.id,d.title,d.category,COALESCE(ch.confidence,d.confidence),d.review_at,d.updated_at,last_review.reviewed_at,
		((EXISTS(SELECT 1 FROM decision_expectations x WHERE x.decision_id=d.id AND x.expected_at<=now()) AND NOT EXISTS(SELECT 1 FROM decision_outcomes o WHERE o.decision_id=d.id)) OR COALESCE(d.review_at<=now(),false)),
		EXISTS(SELECT 1 FROM decision_evidence e WHERE e.decision_id=d.id AND e.known_at>COALESCE(last_review.reviewed_at,d.decided_at)),
		EXISTS(SELECT 1 FROM decision_assumptions a JOIN LATERAL(SELECT status FROM assumption_events ae WHERE ae.assumption_id=a.id ORDER BY ae.known_at DESC,ae.created_at DESC LIMIT 1) st ON true WHERE a.decision_id=d.id AND st.status IN ('WEAKENING','BROKEN')),
		COALESCE(last_review.reviewed_at,d.decided_at)<now()-interval '90 days',
		EXISTS(SELECT 1 FROM invalidation_conditions i JOIN LATERAL(SELECT status FROM invalidation_events ie WHERE ie.condition_id=i.id ORDER BY ie.known_at DESC,ie.created_at DESC LIMIT 1) st ON true WHERE i.decision_id=d.id AND st.status='TRIGGERED'),
		EXISTS(SELECT 1 FROM decision_confidence_history h WHERE h.decision_id=d.id AND h.confidence>=80)
		FROM decisions d
		LEFT JOIN LATERAL(SELECT confidence FROM decision_confidence_history h WHERE h.decision_id=d.id ORDER BY h.recorded_at DESC,h.created_at DESC LIMIT 1) ch ON true
		LEFT JOIN LATERAL(SELECT reviewed_at FROM review_events r WHERE r.decision_id=d.id AND r.event_type='REVIEWED' ORDER BY r.reviewed_at DESC LIMIT 1) last_review ON true
		WHERE d.status IN ('active','draft') AND ($2 OR d.owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))`, actor.ID, actor.IsAdmin())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ReviewItem{}
	for rows.Next() {
		var item domain.ReviewItem
		var signals reviewSignals
		var highConfidenceEver bool
		if err := rows.Scan(&item.DecisionID, &item.Title, &item.Category, &item.Confidence, &item.ReviewAt, &item.UpdatedAt, &item.LastReviewed, &signals.OutcomeDue, &signals.NewEvidence, &signals.AssumptionAtRisk, &signals.LongTimeNoReview, &signals.InvalidationFired, &highConfidenceEver); err != nil {
			return nil, err
		}
		signals.HighConfidence = highConfidenceEver && (item.LastReviewed == nil || signals.OutcomeDue || signals.NewEvidence || signals.AssumptionAtRisk || signals.LongTimeNoReview || signals.InvalidationFired)
		item.Priority, item.Reasons = reviewPriority(signals)
		item.Health = reviewHealth(item.Priority, signals)
		if item.Priority > 0 {
			result = append(result, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority == result[j].Priority {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].Priority > result[j].Priority
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) ReviewDecision(ctx context.Context, actor domain.User, decisionID uuid.UUID, note string, confidence *int, reviewedAt time.Time, nextReviewAt *time.Time) (domain.ReviewItem, error) {
	if err := s.ensureDecisionEditor(ctx, actor, decisionID); err != nil {
		return domain.ReviewItem{}, err
	}
	if strings.TrimSpace(note) == "" {
		return domain.ReviewItem{}, ErrValidation
	}
	if confidence != nil && (*confidence < 0 || *confidence > 100) {
		return domain.ReviewItem{}, ErrValidation
	}
	if reviewedAt.IsZero() {
		reviewedAt = time.Now().UTC()
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.ReviewItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO review_events(id,decision_id,reviewer_id,event_type,note,confidence,reviewed_at,next_review_at) VALUES($1,$2,$3,'REVIEWED',$4,$5,$6,$7)`, uuid.New(), decisionID, actor.ID, strings.TrimSpace(note), confidence, reviewedAt.UTC(), nextReviewAt); err != nil {
		return domain.ReviewItem{}, err
	}
	if confidence != nil {
		if err := insertConfidenceRecord(ctx, tx, actor.ID, decisionID, *confidence, "Review: "+strings.TrimSpace(note), reviewedAt); err != nil {
			return domain.ReviewItem{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE decisions SET confidence=$2,updated_at=now() WHERE id=$1`, decisionID, *confidence); err != nil {
			return domain.ReviewItem{}, err
		}
	}
	if nextReviewAt != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO review_schedules(id,decision_id,next_review_at,created_by) VALUES($1,$2,$3,$4) ON CONFLICT(decision_id) DO UPDATE SET next_review_at=excluded.next_review_at,enabled=true,updated_at=now()`, uuid.New(), decisionID, nextReviewAt, actor.ID); err != nil {
			return domain.ReviewItem{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE decisions SET review_at=$2,updated_at=now() WHERE id=$1`, decisionID, nextReviewAt); err != nil {
			return domain.ReviewItem{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{"note": strings.TrimSpace(note), "confidence": confidence, "nextReviewAt": nextReviewAt})
	if _, err := tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,actor_id,event_type,payload,effective_at,known_at) VALUES($1,$2,$3,'decision.reviewed',$4,$5,$5)`, uuid.New(), decisionID, actor.ID, payload, reviewedAt.UTC()); err != nil {
		return domain.ReviewItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ReviewItem{}, err
	}
	items, err := s.ReviewQueue(ctx, actor, 100)
	if err != nil {
		return domain.ReviewItem{}, err
	}
	for _, item := range items {
		if item.DecisionID == decisionID {
			return item, nil
		}
	}
	var title, category string
	var currentConfidence int
	err = s.DB.QueryRow(ctx, `SELECT title,category,confidence FROM decisions WHERE id=$1`, decisionID).Scan(&title, &category, &currentConfidence)
	if err == pgx.ErrNoRows {
		return domain.ReviewItem{}, ErrNotFound
	}
	return domain.ReviewItem{DecisionID: decisionID, Title: title, Category: category, Confidence: currentConfidence, Priority: 0, Health: "HEALTHY", Reasons: []string{}, LastReviewed: &reviewedAt, UpdatedAt: time.Now().UTC()}, err
}
