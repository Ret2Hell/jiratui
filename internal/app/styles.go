package app

import "charm.land/lipgloss/v2"

type styles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	HeaderMeta  lipgloss.Style
	StatLabel   lipgloss.Style
	StatValue   lipgloss.Style
	Panel       lipgloss.Style
	PanelActive lipgloss.Style
	Footer      lipgloss.Style
	Muted       lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Selected    lipgloss.Style
	Done        lipgloss.Style
	WIP         lipgloss.Style
	Todo        lipgloss.Style
	Blocked     lipgloss.Style
	InputLabel  lipgloss.Style
}

func newStyles() styles {
	return styles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E0B0FF")),
		Subtitle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		HeaderMeta:  lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")),
		StatLabel:   lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		StatValue:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")),
		Panel:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#334155")).Padding(0, 1),
		PanelActive: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#A78BFA")).Padding(0, 1),
		Footer:      lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Background(lipgloss.Color("#1E293B")),
		Muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")),
		Selected:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#C4B5FD")),
		Done:        lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")),
		WIP:         lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")),
		Todo:        lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")),
		Blocked:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")),
		InputLabel:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CBD5E1")),
	}
}
