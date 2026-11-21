package style

import "testing"

func TestFormat(t *testing.T) {
	cases := []struct{ style, typ, scope, msg, want string }{
		{"conventional", "feat", "auth", "add login", "feat(auth): add login"},
		{"conventional", "feat", "", "add login", "feat: add login"},
		{"plain", "feat", "x", "add login", "add login"},
		{"colon", "fix", "", "typo", "fix: typo"},
		{"bracket", "docs", "", "readme", "[docs] readme"},
		{"{type}({scope}): {message}", "feat", "", "add x", "feat: add x"},
		{"{type}({scope}): {message}", "feat", "api", "add x", "feat(api): add x"},
		{"conventional", "feat", "", "", ""},
	}
	for _, c := range cases {
		if got := Format(c.style, c.typ, c.scope, c.msg); got != c.want {
			t.Errorf("Format(%q,%q,%q,%q) = %q, want %q", c.style, c.typ, c.scope, c.msg, got, c.want)
		}
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("conventional") || !IsValid("{type}: {message}") || IsValid("nonsense") {
		t.Error("IsValid misbehaving")
	}
}