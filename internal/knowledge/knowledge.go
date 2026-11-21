package knowledge

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed rules.txt examples.txt bad_examples.txt
var defaults embed.FS

type Knowledge struct {
	Rules       string
	Examples    string
	BadExamples string
}

func Load(overrideDirs ...string) Knowledge {
	return Knowledge{
		Rules:       pick("rules.txt", overrideDirs),
		Examples:    pick("examples.txt", overrideDirs),
		BadExamples: pick("bad_examples.txt", overrideDirs),
	}
}

func pick(name string, dirs []string) string {
	for _, d := range dirs {
		if d == "" {
			continue
		}

		if s := readFile(filepath.Join(d, name)); s != "" {
			return s
		}
	}

	data, err := defaults.ReadFile(name)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}
