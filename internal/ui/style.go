package ui

import "github.com/charmbracelet/lipgloss"

var (
	bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)

	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder())

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

const welcomeBanner = `
 ╔═══════════════════════════════════╗
 ║            commitai               ║
 ║      AI-powered git commits       ║
 ╚═══════════════════════════════════╝
`
