package config

import "testing"

func TestResolveGroq(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "gk")
	cfg := defaults()
	cfg.Backend = "groq"
	if err := Resolve(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != "openai" || cfg.APIBase != groqBase || cfg.APIKey != "gk" || cfg.Model != "llama-3.3-70b-versatile" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestResolveUnknownBackend(t *testing.T) {
	cfg := defaults()
	cfg.Backend = "banana"
	if err := Resolve(&cfg); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestResolveCleansMarkdownLink(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	cfg := defaults()
	cfg.Backend = "openai"
	cfg.APIBase = "[https://api.example.com/v1](https://api.example.com/v1)"
	if err := Resolve(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.APIBase != "https://api.example.com/v1" {
		t.Fatalf("got %q", cfg.APIBase)
	}
}

func TestResolveRejectsPlainHTTP(t *testing.T) {
	cfg := defaults()
	cfg.Backend = "openai"
	cfg.APIBase = "http://example.com/v1"
	if err := Resolve(&cfg); err == nil {
		t.Fatal("expected error for non-local http")
	}
}

func TestResolveClampsSuggestions(t *testing.T) {
	cfg := defaults()
	cfg.Suggestions = 99
	if err := Resolve(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Suggestions != MaxSuggestions {
		t.Fatalf("got %d", cfg.Suggestions)
	}
}
