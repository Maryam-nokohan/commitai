package ai

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func generateLocal(_ context.Context, req Request) (Response, error) {
	var files []*diffFile
	for _, f := range parseDiffFiles(req.Diff) {
		if f.path != "" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return Response{}, errors.New("could not find any changed files in the diff")
	}

	count := req.Suggestions
	if count <= 0 {
		count = 3
	}

	suggestions := buildLocalSuggestions(files, count)
	if len(suggestions) == 0 {
		return Response{}, errors.New("could not derive a commit message from the diff")
	}
	return Response{Suggestions: suggestions}, nil
}

//  diff parsing 

type fileStatus string

const (
	statusAdded    fileStatus = "added"
	statusDeleted  fileStatus = "deleted"
	statusRenamed  fileStatus = "renamed"
	statusModified fileStatus = "modified"
)

type diffFile struct {
	path       string
	oldPath    string
	status     fileStatus
	additions  int
	deletions  int
	addedLines []string
}

var diffHeaderRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)

func parseDiffFiles(diff string) []*diffFile {
	var files []*diffFile
	var cur *diffFile
	inHunk := false

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if cur != nil {
				files = append(files, cur)
			}
			cur = &diffFile{status: statusModified}
			inHunk = false
			if m := diffHeaderRe.FindStringSubmatch(line); m != nil {
				cur.oldPath = strings.Trim(m[1], "\"")
				cur.path = strings.Trim(m[2], "\"")
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}

		if !inHunk {
			switch {
			case strings.HasPrefix(line, "new file mode"):
				cur.status = statusAdded
			case strings.HasPrefix(line, "deleted file mode"):
				cur.status = statusDeleted
			case strings.HasPrefix(line, "rename from "):
				cur.status = statusRenamed
				cur.oldPath = strings.TrimPrefix(line, "rename from ")
			case strings.HasPrefix(line, "rename to "):
				cur.path = strings.TrimPrefix(line, "rename to ")
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "+"):
			cur.additions++
			cur.addedLines = append(cur.addedLines, strings.TrimPrefix(line, "+"))
		case strings.HasPrefix(line, "-"):
			cur.deletions++
		}
	}
	if cur != nil {
		files = append(files, cur)
	}
	return files
}

func (f *diffFile) base() string { return filepath.Base(f.path) }
func (f *diffFile) ext() string  { return strings.ToLower(filepath.Ext(f.path)) }

func (f *diffFile) isTest() bool {
	p := strings.ToLower(f.path)
	return strings.Contains(p, "_test.") ||
		strings.Contains(p, "/test/") || strings.Contains(p, "/tests/") ||
		strings.HasPrefix(filepath.Base(p), "test_") ||
		strings.HasSuffix(p, ".test.js") || strings.HasSuffix(p, ".test.ts") ||
		strings.HasSuffix(p, ".spec.js") || strings.HasSuffix(p, ".spec.ts")
}

func (f *diffFile) isDocs() bool {
	switch f.ext() {
	case ".md", ".rst", ".adoc":
		return true
	}
	if strings.Contains(strings.ToLower(f.path), "docs/") {
		return true
	}
	switch strings.ToUpper(f.base()) {
	case "README", "README.MD", "CHANGELOG", "CHANGELOG.MD", "LICENSE", "CONTRIBUTING.MD":
		return true
	}
	return false
}

func (f *diffFile) isCI() bool {
	p := strings.ToLower(f.path)
	return strings.Contains(p, ".github/workflows/") ||
		strings.Contains(p, ".gitlab-ci") ||
		strings.Contains(p, ".circleci/") ||
		f.base() == "Jenkinsfile"
}

func (f *diffFile) isBuild() bool {
	switch f.base() {
	case "go.mod", "go.sum", "go.work", "package.json", "package-lock.json",
		"yarn.lock", "pnpm-lock.yaml", "Makefile", "Dockerfile", "docker-compose.yml",
		"requirements.txt", "Pipfile", "Pipfile.lock", "Cargo.toml", "Cargo.lock",
		"pom.xml", "build.gradle":
		return true
	}
	return false
}

func allMatch(files []*diffFile, pred func(*diffFile) bool) bool {
	for _, f := range files {
		if !pred(f) {
			return false
		}
	}
	return true
}

var jsIdentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(`),
	regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([A-Za-z0-9_]+)\s*=`),
	regexp.MustCompile(`^(?:export\s+)?class\s+([A-Za-z0-9_]+)`),
}

var identifierPatterns = map[string][]*regexp.Regexp{
	".go": {
		regexp.MustCompile(`^func\s+(?:\([^)]*\)\s+)?([A-Za-z0-9_]+)\s*\(`),
		regexp.MustCompile(`^type\s+([A-Za-z0-9_]+)\s+(?:struct|interface)`),
	},
	".ts":  jsIdentPatterns,
	".tsx": jsIdentPatterns,
	".js":  jsIdentPatterns,
	".jsx": jsIdentPatterns,
	".py": {
		regexp.MustCompile(`^def\s+([A-Za-z0-9_]+)\s*\(`),
		regexp.MustCompile(`^class\s+([A-Za-z0-9_]+)`),
	},
	".rs": {
		regexp.MustCompile(`^(?:pub\s+)?fn\s+([A-Za-z0-9_]+)\s*\(`),
		regexp.MustCompile(`^(?:pub\s+)?struct\s+([A-Za-z0-9_]+)`),
	},
}

func extractIdentifiers(f *diffFile) []string {
	patterns, ok := identifierPatterns[f.ext()]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, line := range f.addedLines {
		trimmed := strings.TrimSpace(line)
		for _, re := range patterns {
			if m := re.FindStringSubmatch(trimmed); m != nil {
				name := m[len(m)-1]
				if name != "" && !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	return names
}

//  classification 

var bugRe = regexp.MustCompile(`(?i)\b(fix(?:es|ed)?|bug|hotfix|panic|crash(?:es)?|nil pointer)\b`)

func classify(files []*diffFile) string {
	if allMatch(files, (*diffFile).isDocs) {
		return "docs"
	}
	if allMatch(files, (*diffFile).isTest) {
		return "test"
	}
	if allMatch(files, (*diffFile).isCI) {
		return "ci"
	}
	if allMatch(files, (*diffFile).isBuild) {
		return "build"
	}

	added, deleted, modified := 0, 0, 0
	totalAdd, totalDel := 0, 0
	bugSignal := false
	for _, f := range files {
		switch f.status {
		case statusAdded:
			added++
		case statusDeleted:
			deleted++
		default:
			modified++
		}
		totalAdd += f.additions
		totalDel += f.deletions
		for _, l := range f.addedLines {
			if bugRe.MatchString(l) {
				bugSignal = true
				break
			}
		}
	}

	switch {
	case added > 0 && added >= modified && deleted == 0:
		return "feat"
	case deleted > 0 && deleted >= added && deleted >= modified:
		return "chore"
	case bugSignal && modified > 0:
		return "fix"
	case totalAdd > totalDel*2:
		return "feat"
	default:
		return "refactor"
	}
}

func alternateType(primary string) string {
	switch primary {
	case "feat", "fix":
		return "refactor"
	case "refactor", "docs":
		return "chore"
	case "chore":
		return "refactor"
	default:
		return ""
	}
}

//  scope + message building 

func commonScope(files []*diffFile) string {
	var dirs [][]string
	for _, f := range files {
		d := filepath.ToSlash(filepath.Dir(f.path))
		if d == "." {
			continue
		}
		dirs = append(dirs, strings.Split(d, "/"))
	}
	if len(dirs) == 0 {
		return ""
	}
	minLen := len(dirs[0])
	for _, d := range dirs {
		if len(d) < minLen {
			minLen = len(d)
		}
	}
	var common []string
	for i := 0; i < minLen; i++ {
		seg := dirs[0][i]
		for _, d := range dirs {
			if d[i] != seg {
				if len(common) == 0 {
					return ""
				}
				return common[len(common)-1]
			}
		}
		common = append(common, seg)
	}
	if len(common) == 0 {
		return ""
	}
	return common[len(common)-1]
}

func friendlyName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return name
}

func verbFor(status fileStatus) string {
	switch status {
	case statusAdded:
		return "add"
	case statusDeleted:
		return "remove"
	case statusRenamed:
		return "rename"
	default:
		return "update"
	}
}

func fileNames(files []*diffFile, max int) string {
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, friendlyName(f.path))
	}
	sort.Strings(names)
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:max], ", ") + fmt.Sprintf(" and %d more", len(names)-max)
}

func buildMessage(files []*diffFile) string {
	if len(files) == 1 {
		f := files[0]
		verb := verbFor(f.status)
		if f.status == statusRenamed {
			return "rename " + friendlyName(f.oldPath) + " to " + friendlyName(f.path)
		}
		if ids := extractIdentifiers(f); len(ids) > 0 {
			n := 2
			if len(ids) < n {
				n = len(ids)
			}
			return verb + " " + strings.Join(ids[:n], ", ")
		}
		return verb + " " + friendlyName(f.path)
	}

	byVerb := map[string][]*diffFile{}
	for _, f := range files {
		v := verbFor(f.status)
		byVerb[v] = append(byVerb[v], f)
	}
	if len(byVerb) == 1 {
		for verb, fs := range byVerb {
			return verb + " " + fileNames(fs, 3)
		}
	}

	var parts []string
	for _, v := range []string{"add", "update", "remove", "rename"} {
		if fs := byVerb[v]; len(fs) > 0 {
			parts = append(parts, v+" "+fileNames(fs, 2))
		}
	}
	return strings.Join(parts, "; ")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func genericSummary(files []*diffFile) string {
	added, deleted := 0, 0
	for _, f := range files {
		switch f.status {
		case statusAdded:
			added++
		case statusDeleted:
			deleted++
		}
	}
	total := len(files)
	loc := "codebase"
	if s := commonScope(files); s != "" {
		loc = s
	}
	switch {
	case added == total:
		return fmt.Sprintf("add %s to %s", plural(total, "new file"), loc)
	case deleted == total:
		return fmt.Sprintf("remove %s from %s", plural(total, "file"), loc)
	default:
		return fmt.Sprintf("update %s in %s", plural(total, "file"), loc)
	}
}

func dedupe(in []Suggestion) []Suggestion {
	seen := map[string]bool{}
	out := make([]Suggestion, 0, len(in))
	for _, s := range in {
		key := s.Type + "|" + s.Scope + "|" + s.Message
		if !seen[key] {
			seen[key] = true
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return in
	}
	return out
}

func buildLocalSuggestions(files []*diffFile, count int) []Suggestion {
	primaryType := classify(files)
	scope := commonScope(files)
	message := truncateRunes(buildMessage(files), 68)

	suggestions := []Suggestion{{Type: primaryType, Scope: scope, Message: message}}

	if scope != "" {
		suggestions = append(suggestions, Suggestion{Type: primaryType, Message: message})
	}
	if alt := alternateType(primaryType); alt != "" && alt != primaryType {
		suggestions = append(suggestions, Suggestion{Type: alt, Scope: scope, Message: message})
	}
	if len(suggestions) < count {
		suggestions = append(suggestions, Suggestion{
			Type: primaryType, Scope: scope, Message: truncateRunes(genericSummary(files), 68),
		})
	}

	suggestions = dedupe(suggestions)
	if len(suggestions) > count {
		suggestions = suggestions[:count]
	}
	return suggestions
}