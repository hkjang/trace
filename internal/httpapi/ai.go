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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
)

const aiPromptVersion = "trace-ai-v2"

type aiStreamInput struct {
	Mode            string     `json:"mode"`
	Prompt          string     `json:"prompt"`
	ReplayAt        *time.Time `json:"replayAt"`
	MaxOutputTokens int        `json:"maxOutputTokens"`
}

func (s *Server) handleAIStream(w http.ResponseWriter, r *http.Request) {
	decisionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input aiStreamInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Mode == "" {
		if strings.HasSuffix(r.URL.Path, "/challenge") {
			input.Mode = "counter"
		} else {
			input.Mode = "review"
		}
	}
	allowed := map[string]bool{"review": true, "counter": true, "counterfactual": true, "assumption": true, "replay": true, "clarify": true}
	if !allowed[input.Mode] {
		writeError(w, store.ErrValidation)
		return
	}
	if input.Mode == "replay" && input.ReplayAt == nil {
		writeError(w, store.ErrValidation)
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
		writeError(w, fmt.Errorf("%w: maxOutputTokens exceeds the configured limit", store.ErrValidation))
		return
	}
	var replayAt *time.Time
	if input.Mode == "replay" {
		replayAt = input.ReplayAt
	}
	decision, err := s.Store.GetDecision(r.Context(), userFrom(r), decisionID, replayAt)
	if err != nil {
		writeError(w, err)
		return
	}
	contextBytes, err := json.Marshal(decision)
	if err != nil {
		writeError(w, err)
		return
	}
	maxContextBytes := settings.ContextWindow * 4
	if maxContextBytes > 0 && len(contextBytes) > maxContextBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]string{"code": "context_too_large", "message": "AI 컨텍스트가 설정된 토큰 창을 초과합니다."}})
		return
	}
	sum := sha256.Sum256(contextBytes)
	snapshotHash := hex.EncodeToString(sum[:])
	runID, err := s.Store.StartAnalysisRun(r.Context(), userFrom(r).ID, &decisionID, input.Mode, settings.Model, aiPromptVersion, fmt.Sprintf("decision-v%d", decision.Version), snapshotHash, replayAt)
	if err != nil {
		writeError(w, err)
		return
	}
	prompt := buildAIPrompt(settings, input, contextBytes)
	upstreamBody := map[string]any{"model": settings.Model, "stream": true}
	endpoint := settings.BaseURL + "/responses"
	if settings.Protocol == "responses" {
		upstreamBody["instructions"] = settings.SystemPrompt
		upstreamBody["input"] = prompt
		upstreamBody["max_output_tokens"] = maxTokens
	} else {
		endpoint = settings.BaseURL + "/chat/completions"
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
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]string{"code": "ai_unavailable", "message": "AI 제공자에 연결할 수 없습니다."}})
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
		s.Logger.Error("AI upstream rejected request", "status", response.Status, "detail", string(detail))
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, "upstream rejected request")
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]string{"code": "ai_upstream_error", "message": "AI 제공자가 요청을 거부했습니다."}})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		_ = s.Store.FailAnalysisRun(context.WithoutCancel(r.Context()), runID, "streaming unsupported")
		writeError(w, errorsNew("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	sendSSE(w, "meta", map[string]any{"model": settings.Model, "mode": input.Mode, "replayAt": replayAt, "maxOutputTokens": maxTokens})
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
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
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
		sendSSE(w, "error", map[string]string{"message": "AI 스트림이 중단되었습니다."})
		flusher.Flush()
		return
	}
	content := output.String()
	insight, saveErr := s.Store.SaveAIInsight(context.WithoutCancel(r.Context()), userFrom(r), decisionID, input.Mode, map[string]any{"text": content}, settings.Model, aiPromptVersion, replayAt, snapshotHash)
	if saveErr != nil {
		s.Logger.Error("save AI insight", "error", saveErr)
	}
	_ = s.Store.CompleteAnalysisRun(context.WithoutCancel(r.Context()), runID, map[string]any{"text": content, "insightId": insight.ID})
	if input.Mode == "review" || input.Mode == "replay" {
		if scoreErr := s.Store.EstimateAndSaveDecisionScore(context.WithoutCancel(r.Context()), decision, &runID); scoreErr != nil {
			s.Logger.Warn("save decision score", "error", scoreErr)
		}
	}
	sendSSE(w, "done", map[string]any{"insightId": insight.ID, "analysisRunId": runID, "inputSnapshotHash": snapshotHash})
	flusher.Flush()
}

func buildAIPrompt(settings domain.AISettings, input aiStreamInput, contextBytes []byte) string {
	instruction := map[string]string{
		"review":         "Evaluate decision quality separately from outcome quality. Explain AI-estimated scores for Evidence Quality, Logic, Alternative Consideration, Risk Awareness, Assumption Quality, Calibration, and Counter Evidence. Identify possible biases without treating scores as objective truth.",
		"counter":        "Act as a rigorous devil's advocate. Construct the strongest evidence-based counterargument and list the specific evidence needed to resolve it.",
		"counterfactual": "Analyze the named alternative as a scenario, separating advantages, disadvantages, and unknowns. Do not claim to predict what certainly would have happened.",
		"assumption":     "Evaluate which assumptions are strengthened, weakening, or broken by the supplied evidence and explain why.",
		"replay":         "Evaluate only the supplied point-in-time context. Never infer or mention facts after replayAt. Contrast decision quality from outcome quality.",
		"clarify":        "Clarify and structure the user's thinking without making the decision for them.",
	}[input.Mode]
	return fmt.Sprintf("Task: %s\nUser request: %s\nDecision context (future data has already been removed when replaying):\n%s", instruction, input.Prompt, string(contextBytes))
}
func extractDelta(data []byte, protocol string) (string, bool) {
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		return "", false
	}
	if protocol == "responses" {
		eventType, _ := value["type"].(string)
		if eventType == "response.output_text.delta" {
			delta, _ := value["delta"].(string)
			return delta, false
		}
		return "", eventType == "response.completed"
	}
	choices, _ := value["choices"].([]any)
	if len(choices) == 0 {
		return "", false
	}
	choice, _ := choices[0].(map[string]any)
	deltaMap, _ := choice["delta"].(map[string]any)
	delta, _ := deltaMap["content"].(string)
	finish := choice["finish_reason"] != nil
	return delta, finish
}
func sendSSE(w io.Writer, event string, value any) {
	raw, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
}
func errorsNew(message string) error { return fmt.Errorf("%s", message) }
