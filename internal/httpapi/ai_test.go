package httpapi

import "testing"

func TestExtractResponsesDelta(t *testing.T) {
	delta, done := extractDelta([]byte(`{"type":"response.output_text.delta","delta":"기록"}`), "responses")
	if delta != "기록" || done {
		t.Fatalf("extractDelta() = %q, %v", delta, done)
	}
	_, done = extractDelta([]byte(`{"type":"response.completed"}`), "responses")
	if !done {
		t.Fatal("completed event should stop the stream")
	}
}

func TestExtractChatCompletionDelta(t *testing.T) {
	delta, done := extractDelta([]byte(`{"choices":[{"delta":{"content":"trace"},"finish_reason":null}]}`), "chat_completions")
	if delta != "trace" || done {
		t.Fatalf("extractDelta() = %q, %v", delta, done)
	}
}
