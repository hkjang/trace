package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
)

type semanticSearchInput struct {
	Query      string     `json:"query"`
	Category   string     `json:"category"`
	Limit      int        `json:"limit"`
	DecisionID *uuid.UUID `json:"decisionId"`
}

type semanticSearchOutput struct {
	Query    string                   `json:"query"`
	Model    string                   `json:"model"`
	Fallback bool                     `json:"fallback"`
	Items    []domain.SimilarDecision `json:"items"`
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}

func (s *Server) providerEmbeddings(ctx context.Context, settings domain.AISettings, inputs []string) ([][]float64, error) {
	if !settings.Enabled || settings.EmbeddingModel == "" {
		return nil, fmt.Errorf("embedding provider is not configured")
	}
	body, _ := json.Marshal(map[string]any{"model": settings.EmbeddingModel, "input": inputs, "encoding_format": "float"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(settings.BaseURL, "/")+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if settings.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+settings.APIKey)
	}
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("embedding provider returned %s: %s", response.Status, string(detail))
	}
	var value struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<20)).Decode(&value); err != nil {
		return nil, err
	}
	result := make([][]float64, len(inputs))
	for _, item := range value.Data {
		if item.Index < 0 || item.Index >= len(result) || len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding provider returned an invalid index")
		}
		result[item.Index] = item.Embedding
	}
	for _, embedding := range result {
		if len(embedding) == 0 {
			return nil, fmt.Errorf("embedding provider returned incomplete data")
		}
	}
	return result, nil
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}

func (s *Server) semanticSearch(ctx context.Context, actor domain.User, input semanticSearchInput, exclude *uuid.UUID) (semanticSearchOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" {
		return semanticSearchOutput{}, store.ErrValidation
	}
	if input.Limit < 1 || input.Limit > 30 {
		input.Limit = 8
	}
	documents, err := s.Store.MemoryDocuments(ctx, actor, 200)
	if err != nil {
		return semanticSearchOutput{}, err
	}
	filtered := make([]store.MemoryDocument, 0, len(documents))
	for _, document := range documents {
		if exclude != nil && document.Decision.ID == *exclude {
			continue
		}
		if input.Category != "" && document.Decision.Category != input.Category {
			continue
		}
		filtered = append(filtered, document)
	}
	model, fallback := "trace-local-v1", true
	vectors := make([][]float64, len(filtered)+1)
	settings, settingsErr := s.Store.GetAISettings(ctx, true)
	if settingsErr == nil && settings.Enabled && settings.EmbeddingModel != "" && len(filtered) > 0 {
		timeout := time.Duration(settings.RequestTimeoutSec) * time.Second
		embeddingContext, cancel := context.WithTimeout(ctx, timeout)
		remoteVectors := make([][]float64, len(filtered)+1)
		var remoteErr error
		const providerBatchSize = 32
		for start := 0; start < len(filtered); start += providerBatchSize {
			end := start + providerBatchSize
			if end > len(filtered) {
				end = len(filtered)
			}
			inputs := make([]string, 1, end-start+1)
			inputs[0] = truncateRunes(input.Query, 8000)
			for _, document := range filtered[start:end] {
				inputs = append(inputs, truncateRunes(document.Text, 8000))
			}
			batch, batchErr := s.providerEmbeddings(embeddingContext, settings, inputs)
			if batchErr != nil {
				remoteErr = batchErr
				break
			}
			if remoteVectors[0] == nil {
				remoteVectors[0] = batch[0]
			}
			copy(remoteVectors[start+1:end+1], batch[1:])
		}
		cancel()
		if remoteErr == nil {
			vectors, model, fallback = remoteVectors, settings.EmbeddingModel, false
		} else {
			s.Logger.Warn("semantic search embedding fallback", "error", remoteErr)
		}
	}
	if fallback {
		vectors[0] = store.LocalEmbedding(input.Query, 192)
		for index, document := range filtered {
			vectors[index+1] = store.LocalEmbedding(document.Text, 192)
		}
	}
	related := map[uuid.UUID]bool{}
	if input.DecisionID != nil {
		graph, graphErr := s.Store.DecisionGraph(ctx, actor, input.DecisionID, 1, "", nil, 300)
		if graphErr == nil {
			for _, node := range graph.Nodes {
				if node.ID != *input.DecisionID {
					related[node.ID] = true
				}
			}
		}
	}
	now := time.Now().UTC()
	result := semanticSearchOutput{Query: input.Query, Model: model, Fallback: fallback, Items: []domain.SimilarDecision{}}
	for index, document := range filtered {
		similarity := clamp(store.CosineSimilarity(vectors[0], vectors[index+1]), 0, 1)
		ageDays := math.Max(0, now.Sub(document.Decision.UpdatedAt).Hours()/24)
		recency := math.Exp(-ageDays / 365)
		outcomeWeight := 0.5
		if document.LastOutcome != nil {
			outcomeWeight = float64(*document.LastOutcome+2) / 4
		}
		categoryWeight := 0.0
		if input.Category != "" && input.Category == document.Decision.Category {
			categoryWeight = 1
		}
		relationWeight := 0.0
		if related[document.Decision.ID] {
			relationWeight = 1
		}
		contextScore := similarity*0.65 + recency*0.10 + categoryWeight*0.10 + relationWeight*0.05 + outcomeWeight*0.05 + document.Reliability*0.05
		reasons := []string{fmt.Sprintf("의미 유사도 %.0f%%", similarity*100)}
		if relationWeight > 0 {
			reasons = append(reasons, "직접 연결된 판단")
		}
		if document.LastOutcome != nil {
			reasons = append(reasons, "결과 기록 있음")
		}
		result.Items = append(result.Items, domain.SimilarDecision{Decision: document.Decision, Similarity: similarity, ContextScore: contextScore, MatchedExcerpt: document.Excerpt, Reasons: reasons})
		_ = s.Store.SaveDecisionEmbedding(context.WithoutCancel(ctx), document.Decision.ID, model, document.Text, vectors[index+1])
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].ContextScore == result.Items[j].ContextScore {
			return result.Items[i].Decision.UpdatedAt.After(result.Items[j].Decision.UpdatedAt)
		}
		return result.Items[i].ContextScore > result.Items[j].ContextScore
	})
	if len(result.Items) > input.Limit {
		result.Items = result.Items[:input.Limit]
	}
	return result, nil
}

func (s *Server) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	var input semanticSearchInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.semanticSearch(r.Context(), userFrom(r), input, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSimilarDecisions(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	decision, err := s.Store.GetDecision(r.Context(), userFrom(r), id, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	limit := 6
	input := semanticSearchInput{Query: strings.Join([]string{decision.Title, decision.Decision, decision.Reason, decision.Assumptions}, "\n"), Category: decision.Category, Limit: limit, DecisionID: &id}
	result, err := s.semanticSearch(r.Context(), userFrom(r), input, &id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
