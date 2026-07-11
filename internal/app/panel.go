package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type listPosition struct{ Current, Total int }

type panelSpec struct {
	Title         string
	Active        bool
	Content       string
	Width, Height int
	Position      *listPosition
	Scrollbar     *scrollbarState
}

func (m *Model) renderPanelSpec(spec panelSpec) string {
	w, h := max(0, spec.Width), max(0, spec.Height)
	if w == 0 || h == 0 {
		return ""
	}
	if w < 2 || h < 2 {
		return strings.Join(normalizeLines("", w, h), "\n")
	}
	borderStyle, titleStyle := m.styles.PanelBorder, m.styles.PanelTitle
	marker := "  "
	if spec.Active {
		borderStyle, titleStyle, marker = m.styles.PanelActiveBorder, m.styles.PanelActiveTitle, "◆ "
	}
	border := borderStyle.Render
	innerW := max(0, w-4)
	innerH := max(0, h-2)
	lines := slices.Collect(strings.SplitSeq(spec.Content, "\n"))
	if spec.Content == "" {
		lines = nil
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	for i := range lines {
		lines[i] = truncatePlain(lines[i], innerW)
	}
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	topInside := strings.Repeat("─", w-2)
	label := marker + spec.Title + " "
	if w >= 6 && label != "" {
		label = truncatePlain(label, max(0, w-3))
		used := ansiWidth(label)
		topInside = titleStyle.Render(label) + border(strings.Repeat("─", max(0, w-2-used)))
	} else {
		topInside = border(topInside)
	}
	out := []string{border("╭") + topInside + border("╮")}
	thumb, showThumb := scrollbarThumb{}, false
	if spec.Scrollbar != nil {
		thumb, showThumb = calculateScrollbar(innerH, *spec.Scrollbar)
	}
	for i, line := range lines {
		right := "│"
		if showThumb && i >= thumb.Start && i < thumb.Start+thumb.Height {
			right = m.styles.Scrollbar.Render("▐")
		} else {
			right = border(right)
		}
		out = append(out, border("│")+" "+padRight(line, innerW)+" "+right)
	}
	bottomInside := strings.Repeat("─", w-2)
	if spec.Position != nil && spec.Position.Total > 0 {
		meta := formatPosition(*spec.Position)
		// Keep a separator border cell on both sides of metadata.
		if ansiWidth(meta)+2 <= w-2 {
			left := w - 2 - ansiWidth(meta) - 1
			bottomInside = border(strings.Repeat("─", left)) + m.styles.PanelMeta.Render(" "+meta)
		} else {
			bottomInside = border(bottomInside)
		}
	} else {
		bottomInside = border(bottomInside)
	}
	out = append(out, border("╰")+bottomInside+border("╯"))
	return strings.Join(normalizeLines(strings.Join(out, "\n"), w, h), "\n")
}

func formatPosition(p listPosition) string {
	return strconv.Itoa(min(max(0, p.Current), max(0, p.Total-1))+1) + " of " + strconv.Itoa(max(0, p.Total))
}

func ansiWidth(s string) int { return ansi.StringWidth(s) }
