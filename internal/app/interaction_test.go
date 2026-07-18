package app

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
)

func newMainTestModel(t *testing.T, width, height int) *Model {
	t.Helper()
	m := New(configuredTestConfig(), "", nil, nil, "", true)
	m.screen = screenMain
	m.width, m.height = width, height
	m.loading = false
	m.issues = []jira.Issue{
		{Key: "TEST-1", Summary: strings.Repeat("first detailed summary ", 12), IssueType: jira.IssueType{Name: "Task"}},
		{Key: "TEST-2", Summary: "second", IssueType: jira.IssueType{Name: "Task"}},
		{Key: "TEST-3", Summary: "third", IssueType: jira.IssueType{Name: "Task"}},
	}
	m.recalcTotals()
	return m
}

func TestStoryPointSelectionSupportsUnestimated(t *testing.T) {
	if selectedStoryPoints(0) != nil {
		t.Fatal("first story-point option is not unestimated")
	}
	firstEstimate := selectedStoryPoints(1)
	if firstEstimate == nil || *firstEstimate != 0.5 {
		t.Fatalf("first estimate = %v, want 0.5", firstEstimate)
	}
	if pointIndex(nil) != 0 || pointIndex(firstEstimate) != 1 {
		t.Fatalf("point indexes = nil:%d estimate:%d", pointIndex(nil), pointIndex(firstEstimate))
	}

	m := newMainTestModel(t, 120, 20)
	m.issues[0].StoryPoints = new(3.0)
	m.openPoints()
	m.updatePoints(tea.KeyPressMsg(tea.Key{Code: 'u'}), nil)
	if m.issues[0].StoryPoints != nil || m.pointSelected != 0 {
		t.Fatalf("unestimated selection = %v at %d", m.issues[0].StoryPoints, m.pointSelected)
	}
	m.issues[0].StoryPoints = new(5.0)
	m.openPoints()
	m.updatePoints(tea.KeyPressMsg(tea.Key{Code: 'c'}), nil)
	if m.issues[0].StoryPoints != nil {
		t.Fatalf("clear shortcut left story points at %v", m.issues[0].StoryPoints)
	}
	if got := pointsString(nil); got != "—" {
		t.Fatalf("unestimated label = %q", got)
	}
}

func TestPendingTaskContentSurvivesRefreshAndSuccess(t *testing.T) {
	m := newMainTestModel(t, 120, 20)
	key := m.issues[0].Key
	m.issues[0].Summary = "Optimistic summary"
	m.issues[0].Description = "Optimistic description"
	m.pendingTaskUpdates[key] = pendingTaskUpdate{
		Original: taskContent{Summary: "Old summary", Description: "Old description"},
		Desired:  taskContent{Summary: "Optimistic summary", Description: "Optimistic description"},
	}
	remote := []jira.Issue{{Key: key, Summary: "Stale summary", Description: "Stale description"}}
	m.mergeSprintData(remote)
	if m.issues[0].Summary != "Optimistic summary" || m.issues[0].Description != "Optimistic description" {
		t.Fatalf("refresh replaced pending content: %#v", m.issues[0])
	}

	journal := tasksave.Journal{
		ID:             "save-test",
		Kind:           tasksave.KindUpdate,
		IssueKey:       key,
		Issue:          m.issues[0],
		Draft:          tasksave.Draft{Summary: "Optimistic summary", Description: jira.PlainDescription("Optimistic description"), WriteSummary: true, WriteDescription: true},
		IssueCreated:   true,
		AddedToSprint:  true,
		ContentUpdated: true,
	}
	m.taskSaves[journal.ID] = journal
	m.Update(taskSaveFinishedMsg{Journal: journal})
	issue, _ := m.issueByKey(key)
	if issue.Summary != "Optimistic summary" || issue.Description != "Optimistic description" {
		t.Fatalf("success did not reapply content: %#v", issue)
	}
	if _, pending := m.pendingTaskUpdates[key]; pending {
		t.Fatal("successful update remained pending")
	}
}

func TestPartialSaveLocksTaskUntilAbandoned(t *testing.T) {
	m := newMainTestModel(t, 120, 20)
	issue := m.issues[0]
	journal, err := tasksave.NewUpdate(issue, tasksave.Draft{
		Summary: "Changed", Description: jira.PlainDescription(issue.Description), WriteSummary: true,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m.restoreTaskSave(journal)
	commands := make(map[commandID]bool)
	for _, binding := range m.activeBindings() {
		commands[binding.Command] = true
	}
	if commands[cmdEdit] || commands[cmdDelete] || commands[cmdPoints] || !commands[cmdRetrySave] || !commands[cmdAbandonSave] {
		t.Fatalf("Partial Save bindings = %#v", commands)
	}

	m.Update(taskSaveAbandonedMsg{Journal: journal})
	restored, _ := m.issueByKey(issue.Key)
	if restored.Summary != issue.Summary {
		t.Fatalf("abandon retained unaccepted summary %q, want %q", restored.Summary, issue.Summary)
	}
}

func TestUnsupportedDescriptionIgnoresBracketedPaste(t *testing.T) {
	m := newMainTestModel(t, 120, 20)
	m.openEdit()
	m.focusCreate(1)
	m.editingDescription = jira.Description{EditorText: "Preserved", Editable: false, UnsupportedReason: "rich text"}
	m.createDescription.SetValue("Preserved")
	m.updatePaste(tea.PasteMsg{Content: "changed"})
	if got := m.createDescription.Value(); got != "Preserved" {
		t.Fatalf("read-only description changed to %q", got)
	}
}

func TestDetailsNavigationUsesPanelWidth(t *testing.T) {
	m := newMainTestModel(t, 120, 12)
	m.focus = focusDetails

	m.navigateBoundary(true)

	contentWidth, pageSize := m.detailsPanelMetrics()
	want := clampOffset(1<<30, len(m.detailsLines(contentWidth)), pageSize)
	if m.detailsViewport.Offset != want {
		t.Fatalf("details offset = %d, want %d", m.detailsViewport.Offset, want)
	}
	if want == 0 {
		t.Fatal("test details must overflow the narrow details panel")
	}
}

func TestFilterInputResetsBothViewports(t *testing.T) {
	m := newMainTestModel(t, 120, 20)
	m.filtering = true
	m.ticketViewport.Offset = 2
	m.detailsViewport.Offset = 4

	m.updateMain(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))

	if m.selected != 0 || m.ticketViewport.Offset != 0 || m.detailsViewport.Offset != 0 {
		t.Fatalf("selection/viewports = %d/%d/%d, want 0/0/0", m.selected, m.ticketViewport.Offset, m.detailsViewport.Offset)
	}
}

func TestRightClickDoesNotChangeFocus(t *testing.T) {
	m := newMainTestModel(t, 120, 20)
	layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	m.focus = focusTickets

	m.updateMouse(tea.MouseClickMsg{X: layout.Details.X + 1, Y: layout.Details.Y + 1, Button: tea.MouseRight})

	if m.focus != focusTickets {
		t.Fatalf("right click changed focus to %v", m.focus)
	}
}

func TestShortFilteredEmptyListKeepsFilterVisible(t *testing.T) {
	m := newMainTestModel(t, 40, 6)
	m.filtering = true
	m.filterInput.SetValue("missing")

	plain := ansi.Strip(m.renderMain())
	if !strings.Contains(plain, "/ missing") {
		t.Fatalf("filter input was clipped from short panel:\n%s", plain)
	}
}

func TestEveryScreenHasExactTinyTerminalFallback(t *testing.T) {
	m := newMainTestModel(t, 15, 4)
	for _, screen := range []screen{screenSetup, screenMain, screenCreate, screenDelete, screenPoints, screenReport, screenHelp, screenTheme} {
		m.screen = screen
		content := m.View().Content
		lines := strings.Split(content, "\n")
		if len(lines) != m.height {
			t.Errorf("screen %v height = %d, want %d", screen, len(lines), m.height)
			continue
		}
		for row, line := range lines {
			if width := ansi.StringWidth(line); width != m.width {
				t.Errorf("screen %v row %d width = %d, want %d", screen, row, width, m.width)
			}
		}
		if !strings.Contains(ansi.Strip(content), "Terminal") {
			t.Errorf("screen %v did not render tiny-terminal fallback", screen)
		}
	}
}

func TestScrolledTicketClickUsesModelIndex(t *testing.T) {
	m := newMainTestModel(t, 40, 8)
	m.issues = make([]jira.Issue, 10)
	for i := range m.issues {
		m.issues[i] = jira.Issue{Key: fmt.Sprintf("TEST-%d", i+1), Summary: "ticket", IssueType: jira.IssueType{Name: "Task"}}
	}
	m.selected = 7
	view := m.View()
	plain := ansi.Strip(view.Content)
	if !strings.Contains(plain, "8 of 10") || !strings.Contains(plain, "▐") {
		t.Fatalf("main panel lacks counter or scrollbar:\n%s", plain)
	}
	var zoneX, zoneY int
	found := false
	for range 1000 {
		if zone := m.zones.Get(fmt.Sprintf("%sticket-%d", m.prefix, 5)); zone != nil {
			zoneX, zoneY, found = zone.StartX, zone.StartY, true
			break
		}
		runtime.Gosched()
	}
	if !found {
		t.Fatal("visible ticket zone was not registered")
	}
	m.updateMouse(tea.MouseClickMsg{X: zoneX, Y: zoneY, Button: tea.MouseLeft})

	if m.selected != 5 {
		t.Fatalf("clicked selection = %d, want 5", m.selected)
	}
}

func TestPanelClickFocusUsesCurrentLayout(t *testing.T) {
	for _, tt := range []struct {
		name          string
		width, height int
	}{
		{"wide", 120, 20},
		{"stacked", 60, 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMainTestModel(t, tt.width, tt.height)
			layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
			m.updateMouse(tea.MouseClickMsg{X: layout.Details.X + 1, Y: layout.Details.Y + 1, Button: tea.MouseLeft})
			if m.focus != focusDetails {
				t.Fatalf("details click focus = %v", m.focus)
			}
			m.updateMouse(tea.MouseClickMsg{X: layout.Tickets.X + 1, Y: layout.Tickets.Y + 1, Button: tea.MouseLeft})
			if m.focus != focusTickets {
				t.Fatalf("tickets click focus = %v", m.focus)
			}
		})
	}
}

func TestMouseWheelRoutesToPanelUnderPointer(t *testing.T) {
	m := newMainTestModel(t, 120, 12)
	m.issues = append(m.issues, m.issues...)
	layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	m.selected = 0
	m.updateMouseWheel(tea.MouseWheelMsg{X: layout.Tickets.X + 1, Y: layout.Tickets.Y + 1, Button: tea.MouseWheelDown})
	if m.focus != focusTickets || m.selected != 3 {
		t.Fatalf("ticket wheel focus/selection = %v/%d", m.focus, m.selected)
	}

	m.updateMouseWheel(tea.MouseWheelMsg{X: layout.Details.X + 1, Y: layout.Details.Y + 1, Button: tea.MouseWheelDown})
	if m.focus != focusDetails || m.detailsViewport.Offset == 0 {
		t.Fatalf("details wheel focus/offset = %v/%d", m.focus, m.detailsViewport.Offset)
	}
}

func TestScreensRenderAtTheirMinimumSize(t *testing.T) {
	m := newMainTestModel(t, 40, 12)
	cases := []struct {
		screen        screen
		width, height int
	}{
		{screenSetup, 40, 12},
		{screenMain, 20, 6},
		{screenCreate, 40, 12},
		{screenDelete, 30, 8},
		{screenPoints, 20, 6},
		{screenReport, 40, 10},
		{screenHelp, 30, 10},
		{screenTheme, 30, 10},
	}
	for _, tt := range cases {
		m.screen, m.width, m.height = tt.screen, tt.width, tt.height
		content := m.View().Content
		if strings.Contains(ansi.Strip(content), "Terminal too small") {
			t.Errorf("screen %v unexpectedly used fallback", tt.screen)
		}
		lines := strings.Split(content, "\n")
		if len(lines) != tt.height {
			t.Errorf("screen %v height = %d, want %d", tt.screen, len(lines), tt.height)
			continue
		}
		for row, line := range lines {
			if width := ansi.StringWidth(line); width != tt.width {
				t.Errorf("screen %v row %d width = %d, want %d", tt.screen, row, width, tt.width)
			}
		}
	}
}

func TestPointsFooterDescribesSelectionControls(t *testing.T) {
	m := newMainTestModel(t, 100, 20)
	m.screen = screenPoints
	plain := ansi.Strip(m.renderBindingFooter())
	for _, want := range []string{"Change: ←/→", "Select: 0-6", "Save: enter", "Cancel: esc"} {
		if !strings.Contains(plain, want) {
			t.Errorf("points footer %q missing %q", plain, want)
		}
	}
}
