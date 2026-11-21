package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrNotARepo = errors.New("not inside a git repository")
var ErrNoStagedChanges = errors.New("no staged changes found; run 'git add' first")

func IsRepo() bool {
	return exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() == nil
}

// Root returns the top-level directory of the current repository.
func Root() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", ErrNotARepo
	}
	return strings.TrimSpace(string(out)), nil
}

func StagedFiles() ([]string, error) {
	if !IsRepo() {
		return nil, ErrNotARepo
	}

	output, err := exec.Command("git", "-c", "core.quotepath=false", "diff", "--cached", "--name-only", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	raw := strings.Split(string(output), "\x00")
	files := make([]string, 0, len(raw))
	for _, file := range raw {
		if file != "" {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, ErrNoStagedChanges
	}
	return files, nil
}

func StagedDiff() (string, error) {
	if !IsRepo() {
		return "", ErrNotARepo
	}

	cmd := exec.Command("git", "-c", "core.quotepath=false", "diff", "--cached",
		"--no-color", "--no-ext-diff", "--no-textconv", "-M")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	diff := stdout.String()
	if strings.TrimSpace(diff) == "" {
		return "", ErrNoStagedChanges
	}
	return diff, nil
}

func SplitByFile(diff string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(diff, "\n")

	var currentPath string
	var buffer strings.Builder

	flush := func() {
		if currentPath != "" {
			result[currentPath] += buffer.String()
		}
		buffer.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			currentPath = extractBPath(line)
		}
		if currentPath != "" {
			buffer.WriteString(line)
			buffer.WriteByte('\n')
		}
	}
	flush()

	return result
}

func extractBPath(line string) string {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return ""
	}

	value := strings.TrimPrefix(line, prefix)
	bIndex := strings.LastIndex(value, " b/")
	if bIndex == -1 {
		return ""
	}

	path := strings.TrimSpace(value[bIndex+3:])
	return strings.Trim(path, "\"")
}

// Commit runs `git commit -m message`. On failure the returned error
// includes git's own output (hook failures, missing identity, etc.).
func Commit(message string) (string, error) {
	cmd := exec.Command("git", "commit", "-m", message)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return output.String(), fmt.Errorf("git commit failed: %w\n%s", err, strings.TrimSpace(output.String()))
	}
	return output.String(), nil
}

func RecentHistory(n int) string {
	if n <= 0 {
		n = 5
	}
	out, err := exec.Command("git", "log", "-n", fmt.Sprintf("%d", n), "--oneline", "--no-color").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}