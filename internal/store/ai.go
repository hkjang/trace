package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
)

func (s *Store) SaveAIInsight(ctx context.Context, actor domain.User, decisionID uuid.UUID, insightType string, content any, model, promptVersion string, replayAt *time.Time, inputHash string) (domain.AIInsight, error) {
	raw, err := json.Marshal(content)
	if err != nil {
		return domain.AIInsight{}, err
	}
	item := domain.AIInsight{ID: uuid.New(), InsightType: insightType, Content: raw, Model: model, PromptVersion: promptVersion, ReplayAt: replayAt, InputSnapshotHash: inputHash, GeneratedAt: time.Now().UTC()}
	_, err = s.DB.Exec(ctx, `INSERT INTO decision_ai_insights(id,decision_id,insight_type,content,model,prompt_version,replay_at,input_snapshot_hash,created_by,generated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, item.ID, decisionID, item.InsightType, raw, item.Model, item.PromptVersion, item.ReplayAt, item.InputSnapshotHash, actor.ID, item.GeneratedAt)
	return item, err
}
