package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const (
	DefaultOllamaURL   = "http://localhost:11434"
	DefaultSuggestions = 3
	MaxSuggestions     = 10

	groqBase   = "https://api.groq.com/openai/v1"
	geminiBase = "https://generativelanguage.googleapis.com/v1beta/openai/"
)

type Config struct {
	Style       string `yaml:"style"`
	Model       string `yaml:"model"`
	Suggestions int    `yaml:"suggestions"`
	Backend     string `yaml:"backend"`
	APIBase     string `yaml:"api_base"`
	APIKey      string `yaml:"api_key"`
	OllamaURL   string `yaml:"ollama_url"`

	BackendSet bool `yaml:"-"`
}

func defaults() Config {
	return Config{
		Style:       "conventional",
		Suggestions: DefaultSuggestions,
		Backend:     "local",
		OllamaURL:   DefaultOllamaURL,
	}
}

func candidateDirs() []string {
	var dirs []string
	if d, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(d, "commitai"))
	}
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".config", "commitai"))
	}
	return dirs
}

func Path() string {
	if p := strings.TrimSpace(os.Getenv("COMMITAI_CONFIG")); p != "" {
		return p
	}
	dirs := candidateDirs()
	for _, d := range dirs {
		p := filepath.Join(d, "config.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if len(dirs) > 0 {
		return filepath.Join(dirs[len(dirs)-1], "config.yml")
	}
	return "config.yml"
}

func Dir() string { return filepath.Dir(Path()) }

func LoadEnv() error {
	dirs := append([]string{Dir()}, candidateDirs()...)
	for _, d := range dirs {
		p := filepath.Join(d, ".env")
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := godotenv.Load(p); err != nil {
			return fmt.Errorf("could not read %s: %w", p, err)
		}
	}
	return nil
}

// Load reads the config file and environment overrides.
func Load() (Config, error) {
	cfg := defaults()
	path := Path()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", path, err)
		}
		var probe struct {
			Backend string `yaml:"backend"`
		}
		_ = yaml.Unmarshal(data, &probe)
		cfg.BackendSet = strings.TrimSpace(probe.Backend) != ""
	case os.IsNotExist(err):
	default:
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}

	if strings.TrimSpace(cfg.Backend) == "" {
		cfg.Backend = "local"
	}
	if cfg.Suggestions <= 0 {
		cfg.Suggestions = DefaultSuggestions
	}
	applyEnv(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("COMMITAI_BACKEND")); v != "" {
		cfg.Backend, cfg.BackendSet = v, true
	}
	if v := strings.TrimSpace(os.Getenv("COMMITAI_MODEL")); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("COMMITAI_STYLE")); v != "" {
		cfg.Style = v
	}
	if v := strings.TrimSpace(os.Getenv("COMMITAI_API_BASE")); v != "" {
		cfg.APIBase = v
	}
	if v := strings.TrimSpace(os.Getenv("COMMITAI_API_KEY")); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("COMMITAI_SUGGESTIONS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Suggestions = n
		}
	}
}

func Resolve(cfg *Config) error {
	cfg.Backend = strings.ToLower(strings.TrimSpace(cfg.Backend))
	cfg.APIBase = cleanURL(cfg.APIBase)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.OllamaURL = strings.TrimSpace(cfg.OllamaURL)

	switch cfg.Backend {
	case "", "local":
		cfg.Backend = "local"
	case "claude":
		cfg.Backend = "anthropic"
	case "groq":
		cfg.Backend = "openai"
		if cfg.APIBase == "" {
			cfg.APIBase = groqBase
		}
	case "gemini":
		cfg.Backend = "openai"
		if cfg.APIBase == "" {
			cfg.APIBase = geminiBase
		}
	case "openai", "anthropic", "ollama":
	default:
		return fmt.Errorf("unknown backend %q (valid: local, openai, groq, gemini, anthropic, ollama)", cfg.Backend)
	}

	if cfg.Suggestions <= 0 {
		cfg.Suggestions = DefaultSuggestions
	} else if cfg.Suggestions > MaxSuggestions {
		cfg.Suggestions = MaxSuggestions
	}

	if cfg.Backend == "ollama" && (cfg.OllamaURL == "" || cfg.OllamaURL == DefaultOllamaURL) {
		if h := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); h != "" {
			if !strings.Contains(h, "://") {
				h = "http://" + h
			}
			cfg.OllamaURL = h
		}
	}
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = DefaultOllamaURL
	}

	if cfg.Backend == "openai" {
		if err := validateURL("api_base", cfg.APIBase, false); err != nil {
			return err
		}
	}
	if cfg.Backend == "ollama" {
		if err := validateURL("ollama_url", cfg.OllamaURL, true); err != nil {
			return err
		}
	}

	resolveKey(cfg)
	resolveModel(cfg)
	return nil
}

func cleanURL(s string) string {
	s = strings.TrimSpace(s)
	// tolerate a pasted Markdown link: [url](url)
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]"); i > 0 {
			s = s[1:i]
		}
	}
	return strings.TrimSpace(s)
}

func validateURL(name, raw string, allowHTTP bool) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid %s %q", name, raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		h := u.Hostname()
		if allowHTTP || h == "localhost" || h == "127.0.0.1" || h == "::1" {
			return nil
		}
		return fmt.Errorf("%s must use https (plain http is only allowed for localhost): %q", name, raw)
	}
	return fmt.Errorf("%s must start with https://: %q", name, raw)
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func resolveKey(cfg *Config) {
	if cfg.APIKey != "" {
		return
	}
	switch cfg.Backend {
	case "anthropic":
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		host := hostOf(cfg.APIBase)
		switch {
		case strings.Contains(host, "groq.com"):
			cfg.APIKey = os.Getenv("GROQ_API_KEY")
		case strings.Contains(host, "googleapis.com"):
			cfg.APIKey = os.Getenv("GEMINI_API_KEY")
		case strings.Contains(host, "openrouter.ai"):
			cfg.APIKey = os.Getenv("OPENROUTER_API_KEY")
		case strings.Contains(host, "deepseek.com"):
			cfg.APIKey = os.Getenv("DEEPSEEK_API_KEY")
		case strings.Contains(host, "together"):
			cfg.APIKey = os.Getenv("TOGETHER_API_KEY")
		default:
			cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
}

func resolveModel(cfg *Config) {
	if cfg.Model != "" {
		return
	}
	switch cfg.Backend {
	case "anthropic":
		cfg.Model = "claude-sonnet-4-6"
	case "ollama":
		cfg.Model = "llama3.2"
	case "openai":
		host := hostOf(cfg.APIBase)
		switch {
		case strings.Contains(host, "groq.com"):
			cfg.Model = "llama-3.3-70b-versatile"
		case strings.Contains(host, "googleapis.com"):
			cfg.Model = "gemini-2.5-flash"
		case strings.Contains(host, "deepseek.com"):
			cfg.Model = "deepseek-chat"
		default:
			cfg.Model = "gpt-4o-mini"
		}
	}
}
