package httpapi

import (
	"testing"

	"github.com/hkjang/trace/internal/domain"
)

func TestMCPToolsRequireRBACAndTokenScopes(t *testing.T) {
	user := domain.User{Permissions: []string{"decisions.read", "decisions.write"}}
	readOnly := mcpTools(user, []string{"decisions:read"})
	if len(readOnly) != 3 {
		t.Fatalf("read tool count = %d, want 3", len(readOnly))
	}
	readWrite := mcpTools(user, []string{"decisions:*"})
	if len(readWrite) != 4 || readWrite[3]["name"] != "trace.create_decision" {
		t.Fatalf("unexpected read/write tools: %#v", readWrite)
	}
}
