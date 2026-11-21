package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/maryam-nokohan/commitai/internal/config"
	"github.com/maryam-nokohan/commitai/internal/git"
	"github.com/maryam-nokohan/commitai/internal/style"
	"github.com/maryam-nokohan/commitai/internal/ui"
)

var version = "dev"

func currentVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() { os.Exit(run()) }

func run() int {
	var (
		styleFlag       = flag.String("style", "", "commit style: plain, colon, bracket, conventional, or a template with {type}/{scope}/{message}")
		modelFlag       = flag.String("model", "", "model name, e.g. gpt-4o-mini, claude-sonnet-4-6, llama3.2")
		backendFlag     = flag.String("backend", "", "backend: local (default, offline, no key), openai, groq, gemini, anthropic, ollama")
		apiBaseFlag     = flag.String("api-base", "", "custom API base URL (OpenAI-compatible backends)")
		apiKeyFlag      = flag.String("api-key", "", "API key (prefer env vars: the flag is visible in process listings)")
		suggestionsFlag = flag.Int("suggestions", 0, "number of suggestions to generate (1-10)")
		chooseFlag      = flag.Bool("choose-backend", false, "always show the backend picker")
		configPathFlag  = flag.Bool("config-path", false, "print the config file path and exit")
		versionFlag     = flag.Bool("version", false, "show version and exit")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "commitai %s - AI commit messages from your staged changes\n\n", currentVersion())
		fmt.Fprintf(os.Stderr, "Usage:\n  git add <files>\n  commitai [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfig file: %s\nDocs: https://github.com/maryam-nokohan/commitai\n", config.Path())
	}
	flag.Parse()

	if *versionFlag {
		fmt.Println("commitai", currentVersion())
		return 0
	}
	if *configPathFlag {
		fmt.Println(config.Path())
		return 0
	}

	backendFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "backend" && strings.TrimSpace(f.Value.String()) != "" {
			backendFlagSet = true
		}
	})

	if err := config.LoadEnv(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}

	if v := strings.TrimSpace(*styleFlag); v != "" {
		cfg.Style = v
	}
	if v := strings.TrimSpace(*modelFlag); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(*backendFlag); v != "" {
		cfg.Backend = v
	}
	if v := strings.TrimSpace(*apiBaseFlag); v != "" {
		cfg.APIBase = v
	}
	if v := strings.TrimSpace(*apiKeyFlag); v != "" {
		cfg.APIKey = v
	}
	if *suggestionsFlag > 0 {
		cfg.Suggestions = *suggestionsFlag
	}

	backendExplicit := (cfg.BackendSet || backendFlagSet) && !*chooseFlag

	if err := config.Resolve(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return 1
	}
	if !style.IsValid(cfg.Style) {
		fmt.Fprintf(os.Stderr, "invalid style: %q (use plain, colon, bracket, conventional, or a template containing {message})\n", cfg.Style)
		return 1
	}

	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "error: git is not installed or not in PATH")
		return 1
	}
	if !git.IsRepo() {
		fmt.Fprintln(os.Stderr, "error: not inside a git repository")
		return 1
	}
	if _, err := git.StagedFiles(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if err := ui.Run(context.Background(), cfg, backendExplicit); err != nil {
		if !errors.Is(err, ui.ErrShown) {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}