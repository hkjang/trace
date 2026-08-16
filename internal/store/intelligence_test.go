package store

import (
	"testing"
	"time"

	"github.com/hkjang/trace/internal/domain"
)

func TestLocalEmbeddingSimilarity(t *testing.T) {
	query := LocalEmbedding("PostgreSQL을 AI 플랫폼 데이터베이스로 선택", 192)
	related := LocalEmbedding("AI 서비스의 메인 DB로 PostgreSQL과 pgvector를 채택", 192)
	unrelated := LocalEmbedding("커리어 휴가 일정을 다음 달로 조정", 192)
	if CosineSimilarity(query, query) < 0.999 {
		t.Fatal("identical text must have cosine similarity 1")
	}
	if CosineSimilarity(query, related) <= CosineSimilarity(query, unrelated) {
		t.Fatal("related Korean decision text should rank above unrelated text")
	}
}

func TestReviewPriority(t *testing.T) {
	score, reasons := reviewPriority(reviewSignals{OutcomeDue: true, HighConfidence: true, NewEvidence: true, AssumptionAtRisk: true, LongTimeNoReview: true})
	if score != 100 || len(reasons) != 5 {
		t.Fatalf("unexpected priority: score=%d reasons=%d", score, len(reasons))
	}
	if health := reviewHealth(score, reviewSignals{}); health != "CRITICAL" {
		t.Fatalf("expected CRITICAL, got %s", health)
	}
}

func TestDecisionHealthSeparatesOutcomeFromAssumptions(t *testing.T) {
	decision := domain.Decision{Outcomes: []domain.Outcome{{OutcomeScore: -2}}, AssumptionItems: []domain.Assumption{{Status: "ACTIVE"}}}
	if got := decisionHealth(decision, time.Now()); got != "HEALTHY" {
		t.Fatalf("a bad outcome alone must not make decision health critical: %s", got)
	}
	decision.AssumptionItems[0].Status = "BROKEN"
	if got := decisionHealth(decision, time.Now()); got != "CRITICAL" {
		t.Fatalf("a broken assumption should make decision health critical: %s", got)
	}
}

func TestDecisionHealthUsesReplayClock(t *testing.T) {
	replayAt := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	reviewAt := replayAt.Add(30 * 24 * time.Hour)
	decision := domain.Decision{ReviewAt: &reviewAt}
	if got := decisionHealth(decision, replayAt); got != "HEALTHY" {
		t.Fatalf("a future review date at replay time must be healthy: %s", got)
	}
	if got := decisionHealth(decision, reviewAt.Add(time.Second)); got != "NEEDS_REVIEW" {
		t.Fatalf("a past review date must need review: %s", got)
	}
}
