package style

import "strings"

func IsValid(styleName string) bool {
	styleName = strings.ToLower(strings.TrimSpace(styleName))

	switch styleName {
	case "", "plain", "colon", "bracket", "conventional":
		return true
	default:
		return strings.Contains(styleName, "{message}")
	}
}

func Format(styleName, commitType, scope, message string) string {
	commitType = strings.TrimSpace(commitType)
	scope = strings.TrimSpace(scope)
	message = strings.TrimSpace(message)

	if commitType == "" {
		commitType = "chore"
	}
	if message == "" {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(styleName)) {
	case "", "plain":
		return message

	case "colon":
		return commitType + ": " + message

	case "bracket":
		return "[" + commitType + "] " + message

	case "conventional":
		if scope != "" {
			return commitType + "(" + scope + "): " + message
		}
		return commitType + ": " + message

	default:
		tmpl := strings.TrimSpace(styleName)
		if !strings.Contains(tmpl, "{message}") {
			return message
		}
		if scope == "" {
			tmpl = strings.NewReplacer("({scope})", "", "[{scope}]", "", "{scope}", "").Replace(tmpl)
			tmpl = strings.ReplaceAll(tmpl, "  ", " ")
		}
		result := strings.NewReplacer("{type}", commitType, "{scope}", scope, "{message}", message).Replace(tmpl)
		return strings.TrimSpace(result)
	}
}