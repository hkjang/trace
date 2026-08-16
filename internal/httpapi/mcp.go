package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/version"
)

const mcpProtocolVersion = "2025-11-25"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *Server) mcpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validMCPOrigin(r) {
			writeJSON(w, http.StatusForbidden, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: "invalid Origin"}})
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodDelete {
			w.Header().Set("Allow", "POST")
			writeJSON(w, http.StatusMethodNotAllowed, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: "This stateless server does not expose a standalone SSE stream"}})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		accept := r.Header.Get("Accept")
		if !strings.Contains(accept, "application/json") || !strings.Contains(accept, "text/event-stream") {
			writeJSON(w, http.StatusNotAcceptable, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: "Accept must include application/json and text/event-stream"}})
			return
		}
		protocol := r.Header.Get("MCP-Protocol-Version")
		if protocol != "" && protocol != "2025-03-26" && protocol != "2025-06-18" && protocol != mcpProtocolVersion {
			writeJSON(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: "unsupported MCP protocol version"}})
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="trace-mcp"`)
			writeJSON(w, http.StatusUnauthorized, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32001, Message: "Bearer API token required"}})
			return
		}
		user, scopes, err := s.Store.AuthenticateAPIToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32001, Message: "invalid API token"}})
			return
		}
		var request rpcRequest
		if !decodeJSON(w, r, &request) {
			return
		}
		if request.JSONRPC != "2.0" || request.Method == "" {
			writeJSON(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: -32600, Message: "Invalid Request"}})
			return
		}
		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		response := rpcResponse{JSONRPC: "2.0", ID: request.ID}
		switch request.Method {
		case "initialize":
			var params struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(request.Params, &params)
			negotiated := params.ProtocolVersion
			if negotiated != "2025-03-26" && negotiated != "2025-06-18" && negotiated != mcpProtocolVersion {
				negotiated = mcpProtocolVersion
			}
			response.Result = map[string]any{"protocolVersion": negotiated, "capabilities": map[string]any{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "trace", "version": version.Version}, "instructions": "Trace exposes only decisions visible to the authenticated API token owner. Replay tools enforce known_at filtering in PostgreSQL."}
		case "ping":
			response.Result = map[string]any{}
		case "tools/list":
			response.Result = map[string]any{"tools": mcpTools(user, scopes)}
		case "tools/call":
			result, rpcErr := s.callMCPTool(r, user, scopes, request.Params)
			if rpcErr != nil {
				response.Error = rpcErr
			} else {
				response.Result = result
			}
		default:
			response.Error = &rpcError{Code: -32601, Message: "Method not found"}
		}
		writeJSON(w, http.StatusOK, response)
	})
}

func validMCPOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host) && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
func scopeAllowed(scopes []string, required string) bool {
	parts := strings.SplitN(required, ":", 2)
	for _, scope := range scopes {
		if scope == "*" || scope == required || (len(parts) == 2 && scope == parts[0]+":*") {
			return true
		}
	}
	return false
}

func mcpTools(user domain.User, scopes []string) []map[string]any {
	tools := []map[string]any{}
	if user.Can("decisions.read") && scopeAllowed(scopes, "decisions:read") {
		tools = append(tools, map[string]any{"name": "trace.list_decisions", "title": "List Trace decisions", "description": "List decisions visible to the authenticated user.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"status": map[string]string{"type": "string"}, "query": map[string]string{"type": "string"}}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
		tools = append(tools, map[string]any{"name": "trace.get_decision", "title": "Get a Trace decision", "description": "Get a decision with evidence, expectations, outcomes, reflections, AI insights, and temporal events.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string", "format": "uuid"}}, "required": []string{"id"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
		tools = append(tools, map[string]any{"name": "trace.replay_decision", "title": "Replay a Trace decision", "description": "Return only information known on or before the supplied RFC3339 timestamp.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string", "format": "uuid"}, "at": map[string]string{"type": "string", "format": "date-time"}}, "required": []string{"id", "at"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
		tools = append(tools, map[string]any{"name": "trace.compare_replay", "title": "Compare two Trace replay snapshots", "description": "Compare the exact decision state at two RFC3339 timestamps.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string", "format": "uuid"}, "from": map[string]string{"type": "string", "format": "date-time"}, "to": map[string]string{"type": "string", "format": "date-time"}}, "required": []string{"id", "from", "to"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
		tools = append(tools, map[string]any{"name": "trace.get_decision_graph", "title": "Explore a Trace decision graph", "description": "Return the focus decision and its one or two hop network.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string", "format": "uuid"}, "depth": map[string]any{"type": "integer", "minimum": 1, "maximum": 2}, "at": map[string]string{"type": "string", "format": "date-time"}}, "required": []string{"id"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
		tools = append(tools, map[string]any{"name": "trace.search_memory", "title": "Semantically search Trace memory", "description": "Search decision reasons, evidence, outcomes, and reflections using the configured embedding provider or offline local vectors.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]string{"type": "string"}, "category": map[string]string{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 30}}, "required": []string{"query"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}})
	}
	if user.Can("decisions.write") && scopeAllowed(scopes, "decisions:write") {
		tools = append(tools, map[string]any{"name": "trace.create_decision", "title": "Create a Trace decision", "description": "Create a decision record. When administrator approval workflow is enabled, it is created as a draft.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"title": map[string]string{"type": "string"}, "decision": map[string]string{"type": "string"}, "reason": map[string]string{"type": "string"}, "expectation": map[string]string{"type": "string"}, "confidence": map[string]any{"type": "integer", "minimum": 0, "maximum": 100}, "decidedAt": map[string]string{"type": "string", "format": "date-time"}}, "required": []string{"title", "decision", "confidence"}, "additionalProperties": false}, "annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false}})
	}
	return tools
}

func (s *Server) callMCPTool(r *http.Request, user domain.User, scopes []string, raw json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(raw, &call) != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid tool parameters"}
	}
	structured := any(nil)
	var err error
	switch call.Name {
	case "trace.list_decisions":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct {
			Status string `json:"status"`
			Query  string `json:"query"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		structured, err = s.Store.ListDecisions(r.Context(), user, args.Status, args.Query, 50, 0)
	case "trace.get_decision":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		id, parseErr := uuid.Parse(args.ID)
		if parseErr != nil {
			return nil, &rpcError{Code: -32602, Message: "id must be a UUID"}
		}
		structured, err = s.Store.GetDecision(r.Context(), user, id, nil)
	case "trace.replay_decision":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct {
			ID string `json:"id"`
			At string `json:"at"`
		}
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		id, parseErr := uuid.Parse(args.ID)
		at, timeErr := time.Parse(time.RFC3339, args.At)
		if parseErr != nil || timeErr != nil {
			return nil, &rpcError{Code: -32602, Message: "id and at are invalid"}
		}
		structured, err = s.Store.GetDecision(r.Context(), user, id, &at)
	case "trace.compare_replay":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct{ ID, From, To string }
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		id, parseErr := uuid.Parse(args.ID)
		from, fromErr := time.Parse(time.RFC3339, args.From)
		to, toErr := time.Parse(time.RFC3339, args.To)
		if parseErr != nil || fromErr != nil || toErr != nil {
			return nil, &rpcError{Code: -32602, Message: "id, from, and to are invalid"}
		}
		structured, err = s.Store.CompareReplay(r.Context(), user, id, from, to)
	case "trace.get_decision_graph":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct {
			ID    string `json:"id"`
			Depth int    `json:"depth"`
			At    string `json:"at"`
		}
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		id, parseErr := uuid.Parse(args.ID)
		var at *time.Time
		if args.At != "" {
			parsed, timeErr := time.Parse(time.RFC3339, args.At)
			if timeErr != nil {
				return nil, &rpcError{Code: -32602, Message: "at must be RFC3339"}
			}
			at = &parsed
		}
		if parseErr != nil {
			return nil, &rpcError{Code: -32602, Message: "id must be a UUID"}
		}
		structured, err = s.Store.DecisionGraph(r.Context(), user, &id, args.Depth, "", at, 300)
	case "trace.search_memory":
		if !user.Can("decisions.read") || !scopeAllowed(scopes, "decisions:read") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args semanticSearchInput
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		structured, err = s.semanticSearch(r.Context(), user, args, nil)
	case "trace.create_decision":
		if !user.Can("decisions.write") || !scopeAllowed(scopes, "decisions:write") {
			return nil, &rpcError{Code: -32003, Message: "forbidden"}
		}
		var args struct {
			Title       string `json:"title"`
			Decision    string `json:"decision"`
			Reason      string `json:"reason"`
			Expectation string `json:"expectation"`
			Confidence  int    `json:"confidence"`
			DecidedAt   string `json:"decidedAt"`
		}
		if json.Unmarshal(call.Arguments, &args) != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid arguments"}
		}
		decidedAt := time.Now().UTC()
		if args.DecidedAt != "" {
			if parsed, parseErr := time.Parse(time.RFC3339, args.DecidedAt); parseErr == nil {
				decidedAt = parsed
			} else {
				return nil, &rpcError{Code: -32602, Message: "decidedAt must be RFC3339"}
			}
		}
		input := domain.DecisionInput{Title: args.Title, Decision: args.Decision, Reason: args.Reason, Confidence: args.Confidence, DecidedAt: decidedAt, Status: "active"}
		if args.Expectation != "" {
			input.Expectation = &domain.ExpectationInput{Expectation: args.Expectation}
		}
		structured, err = s.Store.CreateDecision(r.Context(), user, input)
	default:
		return nil, &rpcError{Code: -32602, Message: "Unknown tool"}
	}
	if err != nil {
		return map[string]any{"content": []map[string]string{{"type": "text", "text": err.Error()}}, "isError": true}, nil
	}
	textBytes, _ := json.Marshal(structured)
	return map[string]any{"content": []map[string]string{{"type": "text", "text": string(textBytes)}}, "structuredContent": structured, "isError": false}, nil
}
