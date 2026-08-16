package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hkjang/trace/internal/store"
)

func (s *Server) handlePatternAIStream(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Prompt          string `json:"prompt"`
		MaxOutputTokens int    `json:"maxOutputTokens"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.Store.GetAISettings(r.Context(), true)
	if err != nil {
		writeError(w, err)
		return
	}
	if !settings.Enabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"code": "ai_disabled", "message": "관리자가 AI 연동을 활성화하지 않았습니다."}})
		return
	}
	maxTokens := input.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = settings.MaxOutputTokens
	}
	if maxTokens > settings.MaxOutputTokens || maxTokens > 262144 {
		writeError(w, store.ErrValidation)
		return
	}
	actor := userFrom(r)
	analytics, err := s.Store.Analytics(r.Context(), actor)
	if err != nil {
		writeError(w, err)
		return
	}
	documents, err := s.Store.MemoryDocuments(r.Context(), actor, 50)
	if err != nil {
		writeError(w, err)
		return
	}
	memories := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		memories = append(memories, map[string]any{
			"id": document.Decision.ID, "title": document.Decision.Title, "category": document.Decision.Category,
			"decision": document.Decision.Decision, "confidence": document.Decision.Confidence,
			"evidenceCount": len(document.Decision.Evidence), "alternatives": len(document.Decision.Alternatives),
			"assumptions": document.Decision.AssumptionItems, "outcomes": document.Decision.Outcomes,
			"reflections": document.Decision.Reflections,
		})
	}
	contextValue := map[string]any{"analytics": analytics, "decisions": memories}
	contextBytes, _ := json.Marshal(contextValue)
	if settings.ContextWindow > 0 && len(contextBytes) > settings.ContextWindow*4 {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]string{"code": "context_too_large", "message": "패턴 분석 컨텍스트가 설정된 토큰 창을 초과합니다."}})
		return
	}
	sum := sha256.Sum256(contextBytes)
	snapshotHash := hex.EncodeToString(sum[:])
	runID, err := s.Store.StartAnalysisRun(r.Context(), actor.ID, nil, "pattern", settings.Model, aiPromptVersion, "personal-memory-v2", snapshotHash, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	prompt := fmt.Sprintf("Analyze recurring decision patterns across the supplied personal memory. Separate observations from hypotheses, compare categories, identify repeated evidence/risk/confidence/alternative/bias patterns, and give specific examples without declaring personality traits as facts. User request: %s\nContext:\n%s", input.Prompt, contextBytes)
	upstreamBody := map[string]any{"model": settings.Model, "stream": true}
	endpoint := strings.TrimRight(settings.BaseURL, "/") + "/responses"
	if settings.Protocol == "responses" {
		upstreamBody["instructions"] = settings.SystemPrompt
		upstreamBody["input"] = prompt
		upstreamBody["max_output_tokens"] = maxTokens
	} else {
		endpoint = strings.TrimRight(settings.BaseURL, "/") + "/chat/completions"
		upstreamBody["messages"] = []map[string]string{{"role": "system", "content": settings.SystemPrompt}, {"role": "user", "content": prompt}}
		upstreamBody["max_tokens"] = maxTokens
	}
	rawBody, _ := json.Marshal(upstreamBody)
	timeout := time.Duration(settings.RequestTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, err.Error())
		writeError(w, err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if settings.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+settings.APIKey)
	}
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]string{"code": "ai_unavailable", "message": "AI 제공자에 연결할 수 없습니다."}})
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, string(detail))
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]string{"code": "ai_upstream_error", "message": "AI 제공자가 패턴 분석 요청을 거부했습니다."}})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, "streaming unsupported")
		writeError(w, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	sendSSE(w, "meta", map[string]any{"model": settings.Model, "mode": "pattern", "decisionCount": len(memories), "analysisRunId": runID})
	flusher.Flush()
	reader := bufio.NewScanner(response.Body)
	reader.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var output strings.Builder
	for reader.Scan() {
		line := reader.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			if data == "[DONE]" {
				break
			}
			continue
		}
		delta, done := extractDelta([]byte(data), settings.Protocol)
		if delta != "" {
			output.WriteString(delta)
			sendSSE(w, "delta", map[string]string{"text": delta})
			flusher.Flush()
		}
		if done {
			break
		}
	}
	if err := reader.Err(); err != nil {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, err.Error())
		sendSSE(w, "error", map[string]string{"message": "AI 패턴 스트림이 중단되었습니다."})
		flusher.Flush()
		return
	}
	_ = s.Store.CompleteAnalysisRun(context.WithoutCancel(r.Context()), runID, map[string]any{"text": output.String(), "decisionCount": len(memories)})
	sendSSE(w, "done", map[string]any{"analysisRunId": runID, "inputSnapshotHash": snapshotHash})
	flusher.Flush()
}
