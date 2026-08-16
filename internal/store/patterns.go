package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
)

func level(value, medium, high float64) string {
	if value >= high {
		return "HIGH"
	}
	if value >= medium {
		return "MEDIUM"
	}
	return "LOW"
}

func (s *Store) EstimateAndSaveDecisionScore(ctx context.Context, decision domain.Decision, analysisRunID *uuid.UUID) error {
	reliabilityTotal, reliabilityCount, against := 0, 0, 0
	for _, evidence := range decision.Evidence {
		if evidence.Reliability != nil {
			reliabilityTotal += *evidence.Reliability
			reliabilityCount++
		}
		if evidence.Stance == "against" {
			against++
		}
	}
	evidenceQuality := len(decision.Evidence) * 18
	if reliabilityCount > 0 {
		evidenceQuality = evidenceQuality/2 + (reliabilityTotal/reliabilityCount)/2
	}
	if evidenceQuality > 100 {
		evidenceQuality = 100
	}
	logicQuality := len([]rune(strings.TrimSpace(decision.Reason)))
	if logicQuality > 100 {
		logicQuality = 100
	}
	alternatives := len(decision.Alternatives) * 35
	if alternatives > 100 {
		alternatives = 100
	}
	risk := (len(decision.AssumptionItems) + len(decision.Invalidations)) * 20
	if risk > 100 {
		risk = 100
	}
	assumptionQuality := len(decision.AssumptionItems) * 25
	if assumptionQuality > 100 {
		assumptionQuality = 100
	}
	calibration := 50
	if len(decision.Outcomes) > 0 {
		recordedConfidence := decision.Confidence
		if len(decision.ConfidenceHistory) > 0 {
			recordedConfidence = decision.ConfidenceHistory[0].Confidence
		}
		target := 0
		if decision.Outcomes[len(decision.Outcomes)-1].OutcomeScore > 0 {
			target = 100
		}
		calibration = 100 - int(math.Abs(float64(recordedConfidence-target)))
	}
	counterEvidence := 0
	if len(decision.Evidence) > 0 {
		counterEvidence = against * 100 / len(decision.Evidence)
		if counterEvidence > 30 {
			counterEvidence = 100
		} else {
			counterEvidence = counterEvidence * 100 / 30
		}
	}
	overall := (evidenceQuality + logicQuality + alternatives + risk + assumptionQuality + calibration + counterEvidence) / 7
	_, err := s.DB.Exec(ctx, `INSERT INTO decision_scores(id,decision_id,analysis_run_id,evidence_quality,logic_quality,alternative_consideration,risk_awareness,assumption_quality,calibration,counter_evidence,overall,estimated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, uuid.New(), decision.ID, analysisRunID, evidenceQuality, logicQuality, alternatives, risk, assumptionQuality, calibration, counterEvidence, overall, time.Now().UTC())
	return err
}

func (s *Store) PersonalIntelligence(ctx context.Context, actor domain.User, base domain.Analytics) ([]domain.BiasProfileItem, []domain.PatternInsight, domain.DecisionProfile, error) {
	documents, err := s.MemoryDocuments(ctx, actor, 100)
	if err != nil {
		return nil, nil, domain.DecisionProfile{}, err
	}
	total := len(documents)
	if total == 0 {
		return []domain.BiasProfileItem{}, []domain.PatternInsight{}, domain.DecisionProfile{}, nil
	}
	biasCounts := map[string]int{}
	biasCategories := map[string]map[string]int{}
	var evidenceCount, alternativeCount, againstCount, evidenceTotal, highConfidence, reflected, atRisk, positiveOutcome int
	var decisionLagHours float64
	for _, document := range documents {
		decision := document.Decision
		recordedConfidence := decision.Confidence
		if len(decision.ConfidenceHistory) > 0 {
			recordedConfidence = decision.ConfidenceHistory[0].Confidence
		}
		evidenceCount += len(decision.Evidence)
		alternativeCount += len(decision.Alternatives)
		lag := decision.CreatedAt.Sub(decision.DecidedAt).Hours()
		if lag > 0 {
			decisionLagHours += lag
		}
		if recordedConfidence >= 80 {
			highConfidence++
		}
		if len(decision.Reflections) > 0 {
			reflected++
		}
		for _, evidence := range decision.Evidence {
			evidenceTotal++
			if evidence.Stance == "against" {
				againstCount++
			}
		}
		for _, assumption := range decision.AssumptionItems {
			if assumption.Status == "WEAKENING" || assumption.Status == "BROKEN" {
				atRisk++
			}
		}
		if document.LastOutcome != nil && *document.LastOutcome > 0 {
			positiveOutcome++
		}
		addBias := func(code string) {
			biasCounts[code]++
			if biasCategories[code] == nil {
				biasCategories[code] = map[string]int{}
			}
			biasCategories[code][decision.Category]++
		}
		support, against := 0, 0
		for _, evidence := range decision.Evidence {
			if evidence.Stance == "support" {
				support++
			} else if evidence.Stance == "against" {
				against++
			}
		}
		if support >= 2 && against == 0 {
			addBias("CONFIRMATION_BIAS")
		}
		if recordedConfidence >= 80 && document.LastOutcome != nil && *document.LastOutcome <= 0 {
			addBias("OVERCONFIDENCE")
		}
		if len(decision.Alternatives) == 0 {
			addBias("NARROW_ALTERNATIVES")
		}
		if len(decision.AssumptionItems) == 0 && len(decision.Invalidations) == 0 {
			addBias("RISK_BLIND_SPOT")
		}
	}
	biases := make([]domain.BiasProfileItem, 0, len(biasCounts))
	for code, count := range biasCounts {
		biases = append(biases, domain.BiasProfileItem{BiasType: code, Count: count, Percentage: float64(count) / float64(total) * 100, ByCategory: biasCategories[code]})
	}
	sort.Slice(biases, func(i, j int) bool {
		if biases[i].Count == biases[j].Count {
			return biases[i].BiasType < biases[j].BiasType
		}
		return biases[i].Count > biases[j].Count
	})
	averageEvidence := float64(evidenceCount) / float64(total)
	alternativeRate := float64(alternativeCount) / float64(total)
	againstRate := 0.0
	if evidenceTotal > 0 {
		againstRate = float64(againstCount) / float64(evidenceTotal) * 100
	}
	patterns := []domain.PatternInsight{
		{Code: "EVIDENCE_DEPTH", Title: "근거 수집", Description: fmt.Sprintf("판단당 평균 %.1f개의 근거를 남겼습니다.", averageEvidence), Strength: level(averageEvidence, 1.5, 3)},
		{Code: "ALTERNATIVE_THINKING", Title: "대안 검토", Description: fmt.Sprintf("판단당 평균 %.1f개의 대안을 비교했습니다.", alternativeRate), Strength: level(alternativeRate, 0.5, 1.5)},
		{Code: "COUNTER_EVIDENCE", Title: "반대 근거 균형", Description: fmt.Sprintf("전체 근거 중 %.0f%%가 반대 근거입니다.", againstRate), Strength: level(againstRate, 8, 20)},
		{Code: "ASSUMPTION_STABILITY", Title: "전제 안정성", Description: fmt.Sprintf("현재 약화 또는 붕괴 상태인 전제가 %d개입니다.", atRisk), Strength: map[bool]string{true: "LOW", false: "HIGH"}[atRisk > 0]},
	}
	decisionSpeed := "LOW"
	averageLagHours := decisionLagHours / float64(total)
	if averageLagHours <= 24 {
		decisionSpeed = "HIGH"
	} else if averageLagHours <= 24*7 {
		decisionSpeed = "MEDIUM"
	}
	profile := domain.DecisionProfile{
		EvidenceDriven:      level(averageEvidence, 1.5, 3),
		RiskTolerance:       level(float64(highConfidence)/float64(total)*100, 30, 60),
		AlternativeThinking: level(alternativeRate, 0.5, 1.5),
		DecisionSpeed:       decisionSpeed,
		ConfidenceStyle:     level(base.AverageConfidence, 60, 80),
		ReflectionHabit:     level(float64(reflected)/float64(total)*100, 30, 70),
	}
	if profile.EvidenceDriven == "HIGH" && againstRate < 10 {
		profile.Summary = "근거를 충분히 수집하지만, 방향을 정한 뒤에는 반대 근거가 상대적으로 적은 패턴이 있습니다."
	} else if atRisk > 0 {
		profile.Summary = "현재 전제가 약해진 판단이 있습니다. 새로운 결정 전에 Review Inbox를 먼저 확인해 보세요."
	} else if positiveOutcome > 0 {
		profile.Summary = "결과와 판단 과정을 함께 축적하고 있습니다. 확신 변화 이유를 더 남기면 보정 정확도가 높아집니다."
	} else {
		profile.Summary = "판단 데이터가 쌓이는 중입니다. 대안과 반대 근거를 함께 기록하면 개인 패턴이 더 선명해집니다."
	}
	return biases, patterns, profile, nil
}
