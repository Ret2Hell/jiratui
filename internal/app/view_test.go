package app

import (
	"testing"

	"github.com/Ret2Hell/jiratui/internal/report"
)

func TestStatusPointsGroupsBlockedWorkAsProgress(t *testing.T) {
	model := Model{
		totals: report.PointTotals{
			Total:      21.5,
			Todo:       5,
			InProgress: 8,
			Blocked:    0.5,
			Done:       8,
		},
	}

	got := model.statusPoints()
	want := statusPoints{todo: 5, inProgress: 8.5, done: 8}
	if got != want {
		t.Fatalf("statusPoints() = %+v, want %+v", got, want)
	}
}
