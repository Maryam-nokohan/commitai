package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "rules.txt"), []byte("Rule 1"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "examples.txt"), []byte("Ex 1"), 0o644)

	k := Load(dir)
	if k.Rules != "Rule 1" {
		t.Errorf("expected 'Rule 1', got %q", k.Rules)
	}
	if k.Examples != "Ex 1" {
		t.Errorf("expected 'Ex 1', got %q", k.Examples)
	}
	if k.BadExamples == "" {
		t.Error("expected embedded default for bad examples")
	}
}

func TestLoadEmbeddedDefaults(t *testing.T) {
	k := Load("/definitely/not/a/dir")
	if k.Rules == "" || k.Examples == "" || k.BadExamples == "" {
		t.Fatalf("embedded defaults missing: %+v", k)
	}
}