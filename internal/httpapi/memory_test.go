package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hkjang/trace/internal/domain"
)

func TestProviderEmbeddingsUsesCompatibleBatchContract(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected embedding request: %s %s", r.Method, r.URL.Path)
		}
		var input struct {
			Model          string   `json:"model"`
			Input          []string `json:"input"`
			EncodingFormat string   `json:"encoding_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Model != "embedding-local" || input.EncodingFormat != "float" || len(input.Input) != 2 {
			t.Fatalf("unexpected embedding payload: %#v", input)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"index": 1, "embedding": []float64{0, 1}},
			{"index": 0, "embedding": []float64{1, 0}},
		}})
	}))
	defer provider.Close()

	server := &Server{HTTPClient: provider.Client()}
	vectors, err := server.providerEmbeddings(context.Background(), domain.AISettings{
		Enabled: true, BaseURL: provider.URL, APIKey: "secret", EmbeddingModel: "embedding-local",
	}, []string{"query", "decision"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("provider indexes were not restored: %#v", vectors)
	}
}
