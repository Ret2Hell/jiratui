package app

type rect struct {
	X, Y, Width, Height int
}

type layoutOptions struct {
	WideBreakpoint     int
	TicketsWeight      int
	DetailsWeight      int
	ExpandFocusedPanel bool
	FocusedPanelWeight int
	MinimumWidth       int
	MinimumHeight      int
}

type mainLayout struct {
	Header, Tickets, Details, Footer rect
	Stacked, TicketsOnly, Unusable   bool
}

func defaultLayoutOptions() layoutOptions {
	return layoutOptions{WideBreakpoint: 90, TicketsWeight: 2, DetailsWeight: 1, FocusedPanelWeight: 2, MinimumWidth: 20, MinimumHeight: 6}
}

func calculateMainLayout(width, height, headerHeight int, focus focusArea, opts layoutOptions) mainLayout {
	width, height = max(0, width), max(0, height)
	headerHeight = min(max(0, headerHeight), height)
	footerHeight := min(1, max(0, height-headerHeight))
	l := mainLayout{
		Header: rect{Width: width, Height: headerHeight},
		Footer: rect{Y: height - footerHeight, Width: width, Height: footerHeight},
	}
	if width < opts.MinimumWidth || height < opts.MinimumHeight {
		l.Unusable = true
		return l
	}
	bodyY, bodyH := headerHeight, max(0, height-headerHeight-footerHeight)
	if bodyH < 4 {
		l.Unusable = true
		return l
	}
	tw, dw := max(1, opts.TicketsWeight), max(1, opts.DetailsWeight)
	if opts.ExpandFocusedPanel {
		if focus == focusTickets {
			tw *= max(1, opts.FocusedPanelWeight)
		} else {
			dw *= max(1, opts.FocusedPanelWeight)
		}
	}
	if width >= opts.WideBreakpoint {
		left := weightedShare(width, tw, dw)
		l.Tickets = rect{Y: bodyY, Width: left, Height: bodyH}
		l.Details = rect{X: left, Y: bodyY, Width: width - left, Height: bodyH}
		return l
	}
	l.Stacked = true
	if bodyH < 8 {
		l.TicketsOnly = true
		l.Tickets = rect{Y: bodyY, Width: width, Height: bodyH}
		return l
	}
	top := weightedShare(bodyH, tw, dw)
	// Both framed panels need at least four rows.
	top = min(max(4, top), bodyH-4)
	l.Tickets = rect{Y: bodyY, Width: width, Height: top}
	l.Details = rect{Y: bodyY + top, Width: width, Height: bodyH - top}
	return l
}

func weightedShare(total, first, second int) int {
	if total <= 0 {
		return 0
	}
	return total * max(1, first) / (max(1, first) + max(1, second))
}

func (r rect) contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
