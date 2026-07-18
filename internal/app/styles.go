package app

import (
	"image/color"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/Ret2Hell/jiratui/internal/theme"
)

type palette struct {
	accent      color.Color
	primary     color.Color
	secondary   color.Color
	success     color.Color
	warning     color.Color
	error       color.Color
	selectionFG color.Color
	selectionBG color.Color
}

type styles struct {
	Title             lipgloss.Style
	Subtitle          lipgloss.Style
	HeaderMeta        lipgloss.Style
	StatLabel         lipgloss.Style
	StatValue         lipgloss.Style
	PanelBorder       lipgloss.Style
	PanelActiveBorder lipgloss.Style
	PanelTitle        lipgloss.Style
	PanelActiveTitle  lipgloss.Style
	PanelMeta         lipgloss.Style
	Scrollbar         lipgloss.Style
	Footer            lipgloss.Style
	FooterKey         lipgloss.Style
	FooterSeparator   lipgloss.Style
	ModalSection      lipgloss.Style
	Muted             lipgloss.Style
	Error             lipgloss.Style
	Success           lipgloss.Style
	Selected          lipgloss.Style
	Done              lipgloss.Style
	WIP               lipgloss.Style
	Todo              lipgloss.Style
	Blocked           lipgloss.Style
	InputLabel        lipgloss.Style
}

func paletteFromTheme(t theme.Theme) palette {
	if t.IsDefault() {
		return palette{
			accent:      lipgloss.BrightYellow,
			primary:     lipgloss.BrightWhite,
			secondary:   lipgloss.White,
			success:     lipgloss.BrightGreen,
			warning:     lipgloss.BrightYellow,
			error:       lipgloss.BrightRed,
			selectionFG: lipgloss.BrightWhite,
			selectionBG: lipgloss.BrightBlack,
		}
	}

	return palette{
		accent:      lipgloss.Color(t.Accent),
		primary:     lipgloss.Color(t.BrightFG),
		secondary:   lipgloss.Color(t.FG),
		success:     lipgloss.Color(t.Green),
		warning:     lipgloss.Color(t.Yellow),
		error:       lipgloss.Color(t.Red),
		selectionFG: lipgloss.Color(t.BrightFG),
		selectionBG: lipgloss.Color(t.Accent),
	}
}

func newStyles() styles {
	return stylesFromPalette(paletteFromTheme(theme.Default()))
}

func stylesFromPalette(p palette) styles {
	return styles{
		Title:             lipgloss.NewStyle().Bold(true).Foreground(p.accent),
		Subtitle:          lipgloss.NewStyle().Foreground(p.secondary),
		HeaderMeta:        lipgloss.NewStyle().Foreground(p.primary),
		StatLabel:         lipgloss.NewStyle().Foreground(p.secondary),
		StatValue:         lipgloss.NewStyle().Bold(true).Foreground(p.primary),
		PanelBorder:       lipgloss.NewStyle().Foreground(p.secondary),
		PanelActiveBorder: lipgloss.NewStyle().Foreground(p.accent),
		PanelTitle:        lipgloss.NewStyle().Foreground(p.secondary),
		PanelActiveTitle:  lipgloss.NewStyle().Bold(true).Foreground(p.accent),
		PanelMeta:         lipgloss.NewStyle().Foreground(p.secondary),
		Scrollbar:         lipgloss.NewStyle().Foreground(p.accent),
		Footer:            lipgloss.NewStyle().Foreground(p.primary),
		FooterKey:         lipgloss.NewStyle().Bold(true).Foreground(p.accent),
		FooterSeparator:   lipgloss.NewStyle().Foreground(p.secondary),
		ModalSection:      lipgloss.NewStyle().Bold(true).Foreground(p.success),
		Muted:             lipgloss.NewStyle().Foreground(p.secondary),
		Error:             lipgloss.NewStyle().Foreground(p.error),
		Success:           lipgloss.NewStyle().Foreground(p.success),
		Selected:          lipgloss.NewStyle().Foreground(p.selectionFG).Background(p.selectionBG),
		Done:              lipgloss.NewStyle().Foreground(p.success),
		WIP:               lipgloss.NewStyle().Foreground(p.warning),
		Todo:              lipgloss.NewStyle().Foreground(p.accent),
		Blocked:           lipgloss.NewStyle().Foreground(p.error),
		InputLabel:        lipgloss.NewStyle().Bold(true).Foreground(p.primary),
	}
}

func (m *Model) applyTheme(t theme.Theme) {
	p := paletteFromTheme(t)
	m.styles = stylesFromPalette(p)
	m.spinner.Style = lipgloss.NewStyle().Foreground(p.accent)

	applyTextInputTheme := func(input *textinput.Model) {
		componentStyles := input.Styles()
		componentStyles.Focused.Text = lipgloss.NewStyle().Foreground(p.primary)
		componentStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Focused.Suggestion = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(p.accent)
		componentStyles.Blurred.Text = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.Suggestion = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Cursor.Color = p.accent
		input.SetStyles(componentStyles)
	}
	applyTextInputTheme(&m.filterInput)
	for i := range m.setupInputs {
		applyTextInputTheme(&m.setupInputs[i])
	}
	applyTextInputTheme(&m.createSummary)

	applyTextareaTheme := func(input *textarea.Model) {
		componentStyles := input.Styles()
		componentStyles.Focused.Base = lipgloss.NewStyle()
		componentStyles.Focused.Text = lipgloss.NewStyle().Foreground(p.primary)
		componentStyles.Focused.LineNumber = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(p.accent)
		componentStyles.Focused.CursorLine = lipgloss.NewStyle().Foreground(p.primary)
		componentStyles.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(p.accent)
		componentStyles.Blurred.Base = lipgloss.NewStyle()
		componentStyles.Blurred.Text = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.LineNumber = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.CursorLineNumber = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.CursorLine = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(p.secondary)
		componentStyles.Cursor.Color = p.accent
		input.SetStyles(componentStyles)
	}
	applyTextareaTheme(&m.createDescription)
	applyTextareaTheme(&m.reportEditor)

	m.activeTheme = t
}
