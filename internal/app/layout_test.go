package app

import "testing"

func TestCalculateMainLayoutConservesDimensions(t *testing.T) {
	for _, width := range []int{40, 89, 90, 120} {
		for _, height := range []int{8, 24, 60} {
			l := calculateMainLayout(width, height, 1, focusTickets, defaultLayoutOptions())
			if l.Unusable {
				t.Fatalf("%dx%d unexpectedly unusable", width, height)
			}
			if l.Header.Height+l.Tickets.Height+l.Footer.Height != height && !l.Stacked {
				t.Fatalf("wide height not conserved: %+v", l)
			}
			if l.Stacked && !l.TicketsOnly && l.Header.Height+l.Tickets.Height+l.Details.Height+l.Footer.Height != height {
				t.Fatalf("stacked height not conserved: %+v", l)
			}
			if !l.Stacked && l.Tickets.Width+l.Details.Width != width {
				t.Fatalf("width not conserved: %+v", l)
			}
			if (width < 90) != l.Stacked {
				t.Fatalf("width %d stacked=%v", width, l.Stacked)
			}
		}
	}
}

func TestCalculateMainLayoutFocusedWeight(t *testing.T) {
	opts := defaultLayoutOptions()
	opts.ExpandFocusedPanel = true
	tickets := calculateMainLayout(120, 30, 1, focusTickets, opts)
	details := calculateMainLayout(120, 30, 1, focusDetails, opts)
	if tickets.Tickets.Width <= details.Tickets.Width {
		t.Fatalf("focused tickets did not expand: %d <= %d", tickets.Tickets.Width, details.Tickets.Width)
	}
	if tickets.Tickets.Width+tickets.Details.Width != 120 || details.Tickets.Width+details.Details.Width != 120 {
		t.Fatal("expanded layout lost columns")
	}
}

func TestCalculateMainLayoutTinyAndTicketsOnly(t *testing.T) {
	if !calculateMainLayout(19, 20, 1, focusTickets, defaultLayoutOptions()).Unusable {
		t.Fatal("width below minimum should be unusable")
	}
	l := calculateMainLayout(40, 8, 1, focusTickets, defaultLayoutOptions())
	if !l.TicketsOnly || l.Details.Height != 0 {
		t.Fatalf("short layout = %+v", l)
	}
}
