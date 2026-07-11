package app

type listViewport struct{ Offset int }

type scrollbarState struct{ ContentSize, PageSize, Offset int }

type scrollbarThumb struct{ Start, Height int }

func clampOffset(offset, contentSize, pageSize int) int {
	return min(max(0, offset), max(0, contentSize-max(0, pageSize)))
}

func ensureVisible(offset, selected, contentSize, pageSize int) int {
	if contentSize <= 0 || pageSize <= 0 {
		return 0
	}
	selected = min(max(0, selected), contentSize-1)
	offset = clampOffset(offset, contentSize, pageSize)
	if selected < offset {
		offset = selected
	}
	if selected >= offset+pageSize {
		offset = selected - pageSize + 1
	}
	return clampOffset(offset, contentSize, pageSize)
}

func visibleRange(offset, contentSize, pageSize int) (int, int) {
	offset = clampOffset(offset, contentSize, pageSize)
	return offset, min(contentSize, offset+max(0, pageSize))
}

func pageSelection(selected, deltaPages, contentSize, pageSize int) int {
	if contentSize <= 0 {
		return 0
	}
	return min(max(0, selected+deltaPages*max(1, pageSize)), contentSize-1)
}

func calculateScrollbar(trackSize int, state scrollbarState) (scrollbarThumb, bool) {
	if trackSize < 2 || state.PageSize <= 0 || state.ContentSize <= state.PageSize {
		return scrollbarThumb{}, false
	}
	page := min(state.PageSize, state.ContentSize)
	height := max(1, trackSize*page/state.ContentSize)
	height = min(height, trackSize)
	maxOffset := state.ContentSize - page
	maxStart := trackSize - height
	offset := clampOffset(state.Offset, state.ContentSize, page)
	start := 0
	if maxOffset > 0 {
		start = offset * maxStart / maxOffset
	}
	if offset == maxOffset {
		start = maxStart
	}
	return scrollbarThumb{Start: start, Height: height}, true
}
