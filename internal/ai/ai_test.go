package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const okChatBody = `{
	"choices": [
		{
			"message": {
				"content": "{\"suggestions\": [{\"type\": \"feat\", \"scope\": \"core\", \"message\": \"add new feature\"}]}"
			}
		}
	]
}`

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("diff content", 3, "Rule 1", "Good 1", "Bad 1", "History 1")
	for _, want := range []string{"Rule 1", "Good 1", "Bad 1", "History 1", "diff content"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestOpenAIEndpoint(t *testing.T) {
	cases := map[string]string{
		"":                                       "https://api.openai.com/v1/chat/completions",
		"https://x.io/v1/":                       "https://x.io/v1/chat/completions",
		"https://x.io/v1/chat/completions":       "https://x.io/v1/chat/completions",
	}
	for in, want := range cases {
		if got := openAIEndpoint(in); got != want {
			t.Errorf("openAIEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpenAICompatibleProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okChatBody))
	}))
	defer server.Close()

	resp, err := Generate(context.Background(), Request{
		Diff: "sample diff", Model: "m", Suggestions: 1,
		APIKey: "test-key", APIBase: server.URL, Backend: "openai",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Suggestions) != 1 || resp.Suggestions[0].Message != "add new feature" {
		t.Fatalf("unexpected suggestions: %+v", resp.Suggestions)
	}
}

func TestRetryOnServerError(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { retryDelays = old }()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(okChatBody))
	}))
	defer server.Close()

	_, err := Generate(context.Background(), Request{
		Diff: "d", Suggestions: 1, APIKey: "k", APIBase: server.URL, Backend: "openai",
	})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestAuthErrorIsNotRetried(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer server.Close()

	_, err := Generate(context.Background(), Request{
		Diff: "d", Suggestions: 1, APIKey: "k", APIBase: server.URL, Backend: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "check your API key") {
		t.Fatalf("expected API key hint, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("401 must not be retried, got %d calls", calls)
	}
}

func TestUnknownBackend(t *testing.T) {
	if _, err := Generate(context.Background(), Request{Backend: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSuggestionsJSONMessy(t *testing.T) {
	raw := "<think>hmm</think>\n```json\n{\"suggestions\":[{\"type\":\"fix\",\"message\":\"x\"}]}\n```"
	resp, err := parseSuggestionsJSON(raw)
	if err != nil || len(resp.Suggestions) != 1 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}

	resp, err = parseSuggestionsJSON(`Sure! [{"type":"docs","message":"y"}] hope it helps`)
	if err != nil || len(resp.Suggestions) != 1 || resp.Suggestions[0].Type != "docs" {
		t.Fatalf("bare array: resp=%+v err=%v", resp, err)
	}

	if _, err := parseSuggestionsJSON("not json at all"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSanitize(t *testing.T) {
	in := []Suggestion{
		{Type: "FEAT", Scope: " Auth ", Message: "feat(auth): Added JWT middleware."},
		{Type: "bogus", Message: "Update docs"},
		{Type: "fix", Message: ""},
		{Type: "feat", Scope: "auth", Message: "added JWT middleware"},
	}
	out := sanitize(in, 5)
	if len(out) != 2 {
		t.Fatalf("expected 2 suggestions, got %+v", out)
	}
	if out[0] != (Suggestion{Type: "feat", Scope: "auth", Message: "added JWT middleware"}) {
		t.Errorf("unexpected first: %+v", out[0])
	}
	if out[1].Type != "chore" || out[1].Message != "update docs" {
		t.Errorf("unexpected second: %+v", out[1])
	}
}

func TestLocalBackend(t *testing.T) {
	diff := "diff --git a/internal/foo/bar.go b/internal/foo/bar.go\n" +
		"new file mode 100644\nindex 0000000..1111111\n--- /dev/null\n+++ b/internal/foo/bar.go\n" +
		"@@ -0,0 +1,3 @@\n+package foo\n+\n+func Hello() {}\n"

	resp, err := Generate(context.Background(), Request{Diff: diff, Backend: "local", Suggestions: 3})
	if err != nil {
		t.Fatal(err)
	}
	first := resp.Suggestions[0]
	if first.Type != "feat" || first.Scope != "foo" || first.Message != "add Hello" {
		t.Fatalf("unexpected first suggestion: %+v", first)
	}
}

func TestLocalDocsOnly(t *testing.T) {
	diff := "diff --git a/README.md b/README.md\nindex 1..2 100644\n--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n a\n+b\n"
	resp, err := Generate(context.Background(), Request{Diff: diff, Backend: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Suggestions[0].Type != "docs" {
		t.Fatalf("expected docs, got %+v", resp.Suggestions[0])
	}
}