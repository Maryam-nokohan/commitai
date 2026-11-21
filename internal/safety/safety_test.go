package safety

import (
	"strings"
	"testing"
)

func TestIsSensitive(t *testing.T) {
	for _, p := range []string{".env", "cfg/.env.local", "certs/server.pem", "id_rsa", "db/credentials.json"} {
		if !IsSensitive(p) {
			t.Errorf("%s should be sensitive", p)
		}
	}
	for _, p := range []string{"main.go", "internal/ai/ai.go", "README.md"} {
		if IsSensitive(p) {
			t.Errorf("%s should not be sensitive", p)
		}
	}
}

func TestRedact(t *testing.T) {
	in := "api_key: \"abc123supersecret\"\nAuthorization: Bearer abc.def.ghi\nurl: https://user:pw@host.com/x"
	out := Redact(in)
	for _, leaked := range []string{"abc123supersecret", "abc.def.ghi", "user:pw"} {
		if strings.Contains(out, leaked) {
			t.Errorf("secret %q leaked: %s", leaked, out)
		}
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Error("expected [REDACTED] marker")
	}
}

func TestPrepareSkipsSensitive(t *testing.T) {
	chunks := map[string]string{
		"a.go": "diff --git a/a.go b/a.go\n+x\n",
		".env": "diff --git a/.env b/.env\n+SECRET=1\n",
	}
	cleaned, skipped := Prepare(chunks, 1000)
	if strings.Contains(cleaned, "SECRET=1") {
		t.Error("sensitive content leaked")
	}
	if len(skipped) != 1 || skipped[0] != ".env" {
		t.Errorf("unexpected skipped: %v", skipped)
	}
}

func TestPrepareTruncates(t *testing.T) {
	chunks := map[string]string{
		"a.go": "diff --git a/a.go b/a.go\n" + strings.Repeat("x", 10000),
		"b.go": "diff --git a/b.go b/b.go\n+small\n",
	}
	cleaned, _ := Prepare(chunks, 1000)
	if !strings.Contains(cleaned, "TRUNCATED") {
		t.Error("expected truncation marker")
	}
	if !strings.Contains(cleaned, "b/b.go") {
		t.Error("small file should survive truncation")
	}
	if len(cleaned) > 2000 {
		t.Errorf("output too large: %d", len(cleaned))
	}
}

func TestLimitDiffKeepsUTF8(t *testing.T) {
	out := LimitDiff(strings.Repeat("é", 100), 51)
	if !strings.Contains(out, "TRUNCATED") {
		t.Fatal("expected marker")
	}
	for _, r := range out {
		if r == '\uFFFD' {
			t.Fatal("split a UTF-8 character")
		}
	}
}