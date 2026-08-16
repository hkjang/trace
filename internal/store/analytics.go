package store

import (
	"context"

	"github.com/hkjang/trace/internal/domain"
)

func (s *Store) Analytics(ctx context.Context, actor domain.User) (domain.Analytics, error) {
	rows, err := s.DB.Query(ctx, `SELECT COALESCE((SELECT h.confidence FROM decision_confidence_history h WHERE h.decision_id=d.id ORDER BY h.recorded_at,h.created_at LIMIT 1),d.confidence),(SELECT o.outcome_score FROM decision_outcomes o WHERE o.decision_id=d.id ORDER BY o.outcome_at DESC LIMIT 1),(SELECT o.decision_quality FROM decision_outcomes o WHERE o.decision_id=d.id ORDER BY o.outcome_at DESC LIMIT 1),(SELECT count(*) FROM decision_evidence e WHERE e.decision_id=d.id),EXISTS(SELECT 1 FROM decision_reflections r WHERE r.decision_id=d.id) FROM decisions d WHERE ($2 OR d.owner_id=$1 OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=d.team_id AND tm.user_id=$1))`, actor.ID, actor.IsAdmin())
	if err != nil {
		return domain.Analytics{}, err
	}
	defer rows.Close()
	result := domain.Analytics{}
	type bucket struct{ count, success int }
	buckets := map[int]*bucket{}
	var confidenceSum, evidenceSum int
	var reflections int
	for rows.Next() {
		var confidence, evidence int
		var outcome, quality *int
		var reflected bool
		if err := rows.Scan(&confidence, &outcome, &quality, &evidence, &reflected); err != nil {
			return result, err
		}
		result.TotalDecisions++
		confidenceSum += confidence
		evidenceSum += evidence
		if reflected {
			reflections++
		}
		if outcome != nil {
			key := (confidence / 10) * 10
			if key > 100 {
				key = 100
			}
			if buckets[key] == nil {
				buckets[key] = &bucket{}
			}
			buckets[key].count++
			if *outcome > 0 {
				buckets[key].success++
			}
		}
		if outcome != nil && quality != nil {
			goodDecision := *quality >= 0
			goodOutcome := *outcome >= 0
			if goodDecision && goodOutcome {
				result.Skill++
			} else if goodDecision {
				result.BadLuck++
			} else if goodOutcome {
				result.GoodLuck++
			} else {
				result.Mistake++
			}
		}
	}
	if result.TotalDecisions > 0 {
		result.AverageConfidence = float64(confidenceSum) / float64(result.TotalDecisions)
		result.EvidenceDepth = float64(evidenceSum) / float64(result.TotalDecisions)
		result.ReflectionRate = float64(reflections) / float64(result.TotalDecisions) * 100
	}
	for confidence := 0; confidence <= 100; confidence += 10 {
		if item := buckets[confidence]; item != nil {
			result.Calibration = append(result.Calibration, domain.CalibrationBucket{Confidence: confidence, Count: item.count, SuccessRate: float64(item.success) / float64(item.count) * 100})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	result.Biases, result.Patterns, result.Profile, err = s.PersonalIntelligence(ctx, actor, result)
	return result, err
}
