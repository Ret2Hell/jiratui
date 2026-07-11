package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) renderBindingFooter() string {
	width := max(0, m.width)
	return m.styles.Footer.Render(m.bindingFooterLine(width))
}

func (m *Model) bindingFooterLine(width int) string {
	if width <= 0 {
		return ""
	}
	groups := make([]string, 0)
	for _, b := range m.activeBindings() {
		if !b.Footer || len(b.Keys) == 0 {
			continue
		}
		groups = append(groups, bindingDisplayKey(b)+": "+b.Short)
	}
	prefix, separator := " ", " | "
	line := prefix
	for i, group := range groups {
		addition := group
		if i > 0 {
			addition = separator + group
		}
		if ansi.StringWidth(line+addition) <= width {
			line += addition
			continue
		}
		if ansi.StringWidth(line+" …") <= width {
			line += " …"
		}
		break
	}
	return padRight(truncatePlain(line, width), width)
}

func (m *Model) helpContent() string {
	var lines []string
	for _, b := range m.activeBindingsForHelp() {
		lines = append(lines, "  "+strings.Join(b.Keys, " / ")+"  "+b.Description)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) activeBindingsForHelp() []binding {
	copy := *m
	copy.screen = screenMain
	return copy.activeBindings()
}
