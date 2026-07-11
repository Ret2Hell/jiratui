package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) renderBindingFooter() string {
	return m.bindingFooterLine(max(0, m.width))
}

func (m *Model) bindingFooterLine(width int) string {
	if width <= 0 {
		return ""
	}

	groups := make([]string, 0, 5)
	for _, b := range m.activeBindings() {
		if !b.Footer || len(b.Keys) == 0 {
			continue
		}
		group := m.styles.Footer.Render(b.Short+": ") + m.styles.FooterKey.Render(bindingDisplayKey(b))
		groups = append(groups, group)
	}

	line := " "
	separator := m.styles.FooterSeparator.Render(" | ")
	for i, group := range groups {
		addition := group
		if i > 0 {
			addition = separator + group
		}
		if ansi.StringWidth(line+addition) <= width {
			line += addition
			continue
		}
		ellipsis := m.styles.FooterSeparator.Render(" | …")
		if ansi.StringWidth(line+ellipsis) <= width {
			line += ellipsis
		}
		break
	}
	return padRight(truncatePlain(line, width), width)
}

func compactBindingLine(bindings []binding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.Footer {
			parts = append(parts, b.Short+": "+bindingDisplayKey(b))
		}
	}
	return strings.Join(parts, " | ")
}
