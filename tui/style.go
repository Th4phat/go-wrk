package tui

import "github.com/charmbracelet/lipgloss"

var (
	docStyle          = lipgloss.NewStyle().Margin(1, 2)
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	focusedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	inputDefaultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	placeholderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statusIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	statusRunStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	statusStopStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	statusDoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statusErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	_                 = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	metricKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	metricValStyle    = lipgloss.NewStyle().Bold(true)
	histBarStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	selectedItemStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
)
