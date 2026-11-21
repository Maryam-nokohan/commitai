package safety

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const truncationMarker = "\n[DIFF TRUNCATED BY COMMITAI]\n"
const minPerFile = 300

var deniedPatterns = []string{
	".env", ".env.*", "*.env",
	"*.pem", "*.key", "*.crt", "*.cer", "*.p12", "*.pfx",
	"id_rsa*", "id_ed25519*", "id_dsa*", "id_ecdsa*",
	"*.pgpass",
	".npmrc", ".netrc", ".pypirc",
	"*secret*", "*secrets*", "*credential*", "*credentials*", "*password*",
	"*apikey*", "*api_key*", "*api-key*",
	"*token*",
	"*.keystore", "*.jks",
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key)\s*[:=]\s*["'][^"'\r\n]+["']`),
	regexp.MustCompile(`(?i)(secret[_-]?key)\s*[:=]\s*["'][^"'\r\n]+["']`),
	regexp.MustCompile(`(?i)(client[_-]?secret)\s*[:=]\s*["'][^"'\r\n]+["']`),
	regexp.MustCompile(`(?i)(password)\s*[:=]\s*["'][^"'\r\n]+["']`),
	regexp.MustCompile(`(?i)(token)\s*[:=]\s*["'][^"'\r\n]+["']`),
	regexp.MustCompile(`(?i)(access[_-]?token)\s*[:=]\s*["'][^"'\r\n]+["']`),
	// JSON style: "password": "value"
	regexp.MustCompile(`(?i)"(?:api[_-]?key|secret[_-]?key|client[_-]?secret|password|token|access[_-]?token)"\s*:\s*"[^"\r\n]+"`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)-----BEGIN [^-]+ PRIVATE KEY-----[\s\S]*?-----END [^-]+ PRIVATE KEY-----`),
	// well-known token formats
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`gsk_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	regexp.MustCompile(`AQ\.[A-Za-z0-9_-]{30,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	// credentials embedded in URLs: https://user:pass@host
	regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`),
}

var noiseFiles = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "cargo.lock": true, "pipfile.lock": true,
	"poetry.lock": true, "composer.lock": true, "gemfile.lock": true,
}

func IsSensitive(path string) bool {
	path = filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(path))

	for _, pattern := range deniedPatterns {
		pattern = strings.ToLower(pattern)

		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}

		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
			needle := strings.Trim(pattern, "*")
			if strings.Contains(base, needle) {
				return true
			}
		}
	}
	return false
}

// IsNoise reports generated/lock files whose contents are useless to a model.
func IsNoise(path string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(path)))
	if noiseFiles[base] {
		return true
	}
	return strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".min.css") || strings.HasSuffix(base, ".map")
}

// condense keeps a file's diff header but drops its hunks.
func condense(chunk string) string {
	idx := strings.Index(chunk, "\n@@")
	if idx == -1 {
		return chunk
	}
	return chunk[:idx+1] + "[content omitted: generated or lock file]\n"
}

// FilterDiff drops sensitive files and returns the remaining diff plus the skipped paths.
func FilterDiff(fileChunks map[string]string) (cleaned string, skipped []string) {
	paths := sortedKeys(fileChunks)

	var builder strings.Builder
	for _, path := range paths {
		if IsSensitive(path) {
			skipped = append(skipped, path)
			continue
		}
		builder.WriteString(fileChunks[path])
	}
	return builder.String(), skipped
}

// Prepare is the full pipeline used before anything leaves the machine:
// drop sensitive files, shrink lock files, redact secrets, then fit every
// remaining file into maxChars with a fair per-file budget.
func Prepare(fileChunks map[string]string, maxChars int) (cleaned string, skipped []string) {
	type item struct{ text string }
	var kept []item

	for _, path := range sortedKeys(fileChunks) {
		if IsSensitive(path) {
			skipped = append(skipped, path)
			continue
		}
		text := fileChunks[path]
		if IsNoise(path) {
			text = condense(text)
		}
		kept = append(kept, item{text: Redact(text)})
	}
	if len(kept) == 0 {
		return "", skipped
	}

	total := 0
	for _, k := range kept {
		total += len(k.text)
	}

	budgets := make([]int, len(kept))
	if maxChars <= 0 || total <= maxChars {
		for i, k := range kept {
			budgets[i] = len(k.text)
		}
	} else {
		order := make([]int, len(kept))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return len(kept[order[a]].text) < len(kept[order[b]].text) })

		remaining, left := maxChars, len(kept)
		for _, idx := range order {
			share := remaining / left
			if share < minPerFile {
				share = minPerFile
			}
			b := len(kept[idx].text)
			if b > share {
				b = share
			}
			budgets[idx] = b
			remaining -= b
			left--
		}
	}

	var sb strings.Builder
	for i, k := range kept {
		if budgets[i] < len(k.text) {
			sb.WriteString(cutAt(k.text, budgets[i]))
			sb.WriteString(truncationMarker)
		} else {
			sb.WriteString(k.text)
		}
	}
	return sb.String(), skipped
}

func Redact(text string) string {
	result := text
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			lower := strings.ToLower(match)

			if strings.Contains(lower, "bearer ") {
				return "Bearer [REDACTED]"
			}
			if strings.Contains(match, "PRIVATE KEY") {
				return "[PRIVATE KEY REDACTED]"
			}
			if strings.HasPrefix(match, "://") {
				return "://[REDACTED]@"
			}
			if idx := strings.IndexAny(match, ":="); idx >= 0 {
				return match[:idx+1] + " [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return result
}

// LimitDiff truncates to maxChars bytes without splitting a UTF-8 character.
func LimitDiff(diff string, maxChars int) string {
	if maxChars <= 0 || len(diff) <= maxChars {
		return diff
	}
	return cutAt(diff, maxChars) + truncationMarker
}

func cutAt(s string, n int) string {
	if n >= len(s) {
		return s
	}
	if n <= 0 {
		return ""
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
