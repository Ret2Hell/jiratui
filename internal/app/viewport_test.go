package app

import "testing"

func TestEnsureVisibleAndRange(t *testing.T) {
	tests := []struct{ offset, selected, size, page, want int }{{0, 0, 20, 5, 0}, {0, 5, 20, 5, 1}, {7, 6, 20, 5, 6}, {19, 19, 20, 5, 15}, {9, 2, 3, 10, 0}}
	for _, tt := range tests {
		if got := ensureVisible(tt.offset, tt.selected, tt.size, tt.page); got != tt.want {
			t.Errorf("ensureVisible(%d,%d,%d,%d)=%d want %d", tt.offset, tt.selected, tt.size, tt.page, got, tt.want)
		}
	}
	start, end := visibleRange(18, 20, 5)
	if start != 15 || end != 20 {
		t.Fatalf("range=(%d,%d)", start, end)
	}
}

func TestScrollbarEndpointsAndProportion(t *testing.T) {
	state := scrollbarState{ContentSize: 100, PageSize: 20}
	first, ok := calculateScrollbar(10, state)
	if !ok || first != (scrollbarThumb{Start: 0, Height: 2}) {
		t.Fatalf("first=%+v,%v", first, ok)
	}
	state.Offset = 40
	middle, _ := calculateScrollbar(10, state)
	if middle.Start <= 0 || middle.Start >= 8 {
		t.Fatalf("middle=%+v", middle)
	}
	state.Offset = 80
	last, _ := calculateScrollbar(10, state)
	if last.Start != 8 {
		t.Fatalf("last=%+v", last)
	}
	if _, ok := calculateScrollbar(1, state); ok {
		t.Fatal("one-cell track should hide")
	}
	if _, ok := calculateScrollbar(10, scrollbarState{ContentSize: 5, PageSize: 5}); ok {
		t.Fatal("fitting content should hide")
	}
}

func TestPageSelectionPartialFinalPage(t *testing.T) {
	if got := pageSelection(6, 1, 10, 4); got != 9 {
		t.Fatalf("got %d", got)
	}
	if got := pageSelection(2, -1, 10, 4); got != 0 {
		t.Fatalf("got %d", got)
	}
}
