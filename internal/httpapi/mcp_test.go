package httpapi

import (
	"testing"

	"github.com/hkjang/trace/internal/domain"
)

func TestMCPToolsRequireRBACAndTokenScopes(t *testing.T) {
	user := domain.User{Permissions: []string{"decisions.read", "decisions.write"}}
	readOnly := mcpTools(user, []string{"decisions:read"})
	if len(readOnly) != 6 {
		t.Fatalf("read tool count = %d, want 6", len(readOnly))
	}
	readWrite := mcpTools(user, []string{"decisions:*"})
	if len(readWrite) != 7 || readWrite[6]["name"] != "trace.create_decision" {
		t.Fatalf("unexpected read/write tools: %#v", readWrite)
	}
}
