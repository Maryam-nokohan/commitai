package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/maryam-nokohan/commitai/internal/ai"
	"github.com/maryam-nokohan/commitai/internal/config"
	"github.com/maryam-nokohan/commitai/internal/git"
	"github.com/maryam-nokohan/commitai/internal/knowledge"
	"github.com/maryam-nokohan/commitai/internal/safety"
	"github.com/maryam-nokohan/commitai/internal/style"
)

// ErrShown is returned by Run when the TUI already displayed the error.
var ErrShown = errors.New("error already displayed")

type state int

const (
	stateBackendSelect state = iota
	stateChecking
	stateWarnSensitive
	stateLoading
	stateSelect
	stateEdit
	stateConfirm
	stateCommitting
	stateDone
	stateError
)

type backendOption struct {
	label string
	value string
	desc  string
}

var backendOptions = []backendOption{
	{"Local", "local", "Free, fully offline, no API key needed"},
	{"OpenAI-compatible", "openai", "OpenAI, Groq, Gemini, DeepSeek - needs an API key"},
	{"Anthropic", "anthropic", "Claude - needs ANTHROPIC_API_KEY"},
	{"Ollama", "ollama", "Local model server - needs Ollama running"},
}

const maxDiffChars = 12000

type startGenMsg struct {
	diff    string
	skipped []string
	err     error
}

type suggestionsMsg struct {
	suggestions []ai.Suggestion
	err         error
}

type commitDoneMsg struct {
	output string
	err    error
}

type model struct {
	ctx context.Context
	cfg config.Config

	state   state
	spinner spinner.Model
	input   textinput.Model
	width   int

	backendCursor int

	pendingDiff  string
	skippedFiles []string

	suggestions []ai.Suggestion
	cursor      int

	finalType  string
	finalScope string
	finalMsg   string
	formatted  string

	errMsg       string
	errRetry     bool
	commitOutput string
}

func initialModel(ctx context.Context, cfg config.Config, backendExplicit bool) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	input := textinput.New()
	input.CharLimit = 200
	input.Width = 70

	start := stateBackendSelect
	if backendExplicit {
		start = stateChecking
	}

	cursor := 0
	for i, opt := range backendOptions {
		if strings.EqualFold(opt.value, cfg.Backend) {
			cursor = i
			break
		}
	}

	return model{
		ctx:           ctx,
		cfg:           cfg,
		state:         start,
		spinner:       sp,
		input:         input,
		backendCursor: cursor,
	}
}

func (m model) Init() tea.Cmd {
	if m.state == stateBackendSelect {
		return nil
	}
	return tea.Batch(m.spinner.Tick, m.prepareDiff)
}

func (m model) prepareDiff() tea.Msg {
	diff, err := git.StagedDiff()
	if err != nil {
		return startGenMsg{err: err}
	}

	cleaned, skipped := safety.Prepare(git.SplitByFile(diff), maxDiffChars)
	if strings.TrimSpace(cleaned) == "" {
		return startGenMsg{skipped: skipped, err: errors.New("every staged file was excluded as sensitive; nothing left to analyze")}
	}
	return startGenMsg{diff: cleaned, skipped: skipped}
}

func (m model) generateSuggestionsCmd(diff string) tea.Cmd {
	return func() tea.Msg {
		var dirs []string
		if root, err := git.Root(); err == nil {
			dirs = append(dirs, filepath.Join(root, ".commitai"))
		}
		dirs = append(dirs, filepath.Join(config.Dir(), "knowledge"))
		k := knowledge.Load(dirs...)

		resp, err := ai.Generate(m.ctx, ai.Request{
			Diff:        diff,
			Model:       m.cfg.Model,
			Suggestions: m.cfg.Suggestions,
			APIKey:      m.cfg.APIKey,
			APIBase:     m.cfg.APIBase,
			Backend:     m.cfg.Backend,
			OllamaURL:   m.cfg.OllamaURL,
			Rules:       k.Rules,
			Examples:    k.Examples,
			BadExamples: k.BadExamples,
			GitHistory:  git.RecentHistory(5),
		})
		if err != nil {
			return suggestionsMsg{err: err}
		}
		return suggestionsMsg{suggestions: resp.Suggestions}
	}
}

func (m model) fail(err error, retry bool) model {
	m.state = stateError
	m.errMsg = err.Error()
	m.errRetry = retry
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" && m.state != stateEdit && m.state != stateCommitting {
			return m, tea.Quit
		}

	case startGenMsg:
		if msg.err != nil {
			return m.fail(msg.err, false), nil
		}
		m.pendingDiff = msg.diff
		m.skippedFiles = msg.skipped
		if len(msg.skipped) > 0 {
			m.state = stateWarnSensitive
			return m, nil
		}
		m.state = stateLoading
		return m, m.generateSuggestionsCmd(msg.diff)

	case suggestionsMsg:
		if msg.err != nil {
			return m.fail(msg.err, true), nil
		}
		m.suggestions = msg.suggestions
		m.cursor = 0
		m.state = stateSelect
		return m, nil

	case commitDoneMsg:
		if msg.err != nil {
			return m.fail(msg.err, false), nil
		}
		m.state = stateDone
		m.commitOutput = msg.output
		return m, tea.Quit
	}

	switch m.state {
	case stateWarnSensitive:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.state = stateLoading
				return m, m.generateSuggestionsCmd(m.pendingDiff)
			case "n", "N", "esc", "enter":
				return m, tea.Quit
			}
		}

	case stateBackendSelect:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "up", "k":
				if m.backendCursor > 0 {
					m.backendCursor--
				}
			case "down", "j":
				if m.backendCursor < len(backendOptions)-1 {
					m.backendCursor++
				}
			case "enter":
				m.cfg.Backend = backendOptions[m.backendCursor].value
				if err := config.Resolve(&m.cfg); err != nil {
					return m.fail(err, false), nil
				}
				m.state = stateChecking
				return m, tea.Batch(m.spinner.Tick, m.prepareDiff)
			case "esc":
				return m, tea.Quit
			}
		}

	case stateSelect:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.suggestions)-1 {
					m.cursor++
				}
			case "r":
				m.state = stateLoading
				return m, m.generateSuggestionsCmd(m.pendingDiff)
			case "e":
				m.state = stateEdit
				m.input.SetValue(m.suggestions[m.cursor].Message)
				m.input.CursorEnd()
				m.input.Focus()
				return m, textinput.Blink
			case "enter":
				sel := m.suggestions[m.cursor]
				m.finalType, m.finalScope, m.finalMsg = sel.Type, sel.Scope, sel.Message
				m.formatted = style.Format(m.cfg.Style, m.finalType, m.finalScope, m.finalMsg)
				m.state = stateConfirm
			}
		}

	case stateEdit:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				val := strings.TrimSpace(m.input.Value())
				if val == "" {
					return m, nil
				}
				sel := m.suggestions[m.cursor]
				m.finalType, m.finalScope, m.finalMsg = sel.Type, sel.Scope, val
				m.formatted = style.Format(m.cfg.Style, m.finalType, m.finalScope, m.finalMsg)
				m.state = stateConfirm
				return m, nil
			case "esc":
				m.state = stateSelect
				return m, nil
			}
		}
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case stateConfirm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y", "enter":
				if strings.TrimSpace(m.formatted) == "" {
					m.state = stateSelect
					return m, nil
				}
				message := m.formatted
				m.state = stateCommitting
				return m, func() tea.Msg {
					out, err := git.Commit(message)
					return commitDoneMsg{output: out, err: err}
				}
			case "n", "N", "esc":
				m.state = stateSelect
			}
		}

	case stateError:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && m.errRetry && keyMsg.String() == "r" && m.pendingDiff != "" {
			m.state = stateLoading
			return m, m.generateSuggestionsCmd(m.pendingDiff)
		}
	}

	return m, nil
}

func (m model) wrapWidth() int {
	if m.width > 10 {
		return m.width - 2
	}
	return 76
}

func (m model) View() string {
	var sb strings.Builder
	sb.WriteString(bannerStyle.Render(welcomeBanner) + "\n")

	switch m.state {
	case stateChecking:
		sb.WriteString(m.spinner.View() + " Inspecting staged changes...\n")

	case stateWarnSensitive:
		sb.WriteString(warnStyle.Render("Warning: excluded sensitive files from the AI context:") + "\n")
		for _, f := range m.skippedFiles {
			sb.WriteString(dimStyle.Render("  - "+f) + "\n")
		}
		sb.WriteString("\nProceed with the remaining diff? [y/N]: ")

	case stateBackendSelect:
		sb.WriteString(titleStyle.Render("Choose an AI backend:") + "\n\n")
		for i, opt := range backendOptions {
			line := opt.label + " - " + dimStyle.Render(opt.desc)
			if i == m.backendCursor {
				sb.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				sb.WriteString("  " + line + "\n")
			}
		}
		sb.WriteString("\n" + hintStyle.Render("↑/↓ navigate • enter select • q quit"))
		sb.WriteString("\n" + hintStyle.Render("tip: set \"backend\" in your config file to skip this screen"))

	case stateLoading:
		if strings.EqualFold(m.cfg.Backend, "local") {
			sb.WriteString(m.spinner.View() + " Analyzing the diff locally (no API, no cost)...\n")
		} else {
			label := m.cfg.Backend
			if m.cfg.Model != "" {
				label = m.cfg.Model
			}
			sb.WriteString(m.spinner.View() + " Generating commit suggestions with " + infoStyle.Render(label) + "...\n")
		}

	case stateSelect:
		sb.WriteString(titleStyle.Render("Select a commit suggestion:") + "\n\n")
		for i, sug := range m.suggestions {
			formatted := style.Format(m.cfg.Style, sug.Type, sug.Scope, sug.Message)
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> "+formatted) + "\n")
			} else {
				sb.WriteString("  " + formatted + "\n")
			}
		}
		sb.WriteString("\n" + hintStyle.Render("↑/↓ navigate • enter select • e edit • r regenerate • q quit"))

	case stateEdit:
		sb.WriteString(titleStyle.Render("Edit commit message:") + "\n\n")
		sb.WriteString(m.input.View() + "\n\n")
		sb.WriteString(hintStyle.Render("enter confirm • esc cancel"))

	case stateConfirm:
		sb.WriteString(titleStyle.Render("Commit preview:") + "\n\n")
		sb.WriteString(confirmStyle.Render(m.formatted) + "\n\n")
		sb.WriteString("Create the git commit with this message? [Y/n]: ")

	case stateCommitting:
		sb.WriteString(m.spinner.View() + " Committing...\n")

	case stateDone:
		sb.WriteString(successStyle.Render("✔ Committed successfully.") + "\n\n")
		sb.WriteString(dimStyle.Render(strings.TrimSpace(m.commitOutput)) + "\n")

	case stateError:
		wrap := lipgloss.NewStyle().Width(m.wrapWidth())
		sb.WriteString(errorStyle.Render("Error:") + "\n")
		sb.WriteString(wrap.Render(m.errMsg) + "\n\n")
		if m.errRetry {
			sb.WriteString(hintStyle.Render("r retry • q quit"))
		} else {
			sb.WriteString(hintStyle.Render("press q to quit"))
		}
	}

	return sb.String()
}

func Run(ctx context.Context, cfg config.Config, backendExplicit bool) error {
	p := tea.NewProgram(initialModel(ctx, cfg, backendExplicit))
	final, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := final.(model); ok && fm.state == stateError {
		return ErrShown
	}
	return nil
}