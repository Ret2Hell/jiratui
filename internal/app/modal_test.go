package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Ret2Hell/jiratui/internal/service"
)

func TestMainFooterShowsOnlyPrimaryContextActions(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	footer := ansi.Strip(m.renderBindingFooter())
	for _, want := range []string{"Create: n", "Story points: enter", "To Do: t", "In Progress: p", "Done: d", "Keybindings: ?"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer %q missing %q", footer, want)
		}
	}
	for _, unwanted := range []string{"Edit: e", "Delete: D", "Daily report", "Refresh", "Quit"} {
		if strings.Contains(footer, unwanted) {
			t.Errorf("footer %q unexpectedly contains %q", footer, unwanted)
		}
	}
}

func TestKeybindingsAreLogicallyGrouped(t *testing.T) {
	m := newMainTestModel(t, 120, 40)
	lines := ansi.Strip(strings.Join(m.keybindingLines(80), "\n"))
	for _, heading := range []string{"--- Tasks ---", "--- Workflow ---", "--- Navigation ---", "--- View ---", "--- Filter mode ---", "--- Story points ---", "--- Forms and dialogs ---", "--- Setup ---", "--- Keybindings popup ---", "--- Application ---"} {
		if !strings.Contains(lines, heading) {
			t.Errorf("keybindings missing heading %q", heading)
		}
	}
	for _, action := range []string{"create a new task", "delete the selected task", "move to In Progress", "move one page down", "save the report draft", "show all keybindings"} {
		if !strings.Contains(lines, action) {
			t.Errorf("keybindings missing action %q", action)
		}
	}
}

func TestKeybindingsModalOverlaysCurrentScreen(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	backgroundTop := strings.Split(m.renderMain(), "\n")[0]
	m.modalParent = screenMain
	m.screen = screenHelp

	modal := m.renderKeybindingsModal()

	if top := strings.Split(modal, "\n")[0]; top != backgroundTop {
		t.Fatalf("modal replaced background top row\n got: %q\nwant: %q", top, backgroundTop)
	}
	plain := ansi.Strip(modal)
	if !strings.Contains(plain, "Keybindings") || !strings.Contains(plain, "--- Tasks ---") {
		t.Fatalf("keybindings popup not rendered over background:\n%s", plain)
	}
}

func TestCreatePopupMatchesCommitEditorGeometry(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	summary, description := m.createPopupRects()
	if summary.Height != 3 {
		t.Fatalf("summary height = %d, want 3", summary.Height)
	}
	if description.Height < 7 {
		t.Fatalf("description height = %d, want at least 7", description.Height)
	}
	if summary.Width != description.Width || summary.X != description.X || summary.Y+summary.Height != description.Y {
		t.Fatalf("panels are not aligned and adjacent: summary=%+v description=%+v", summary, description)
	}
}

func TestCreateModalUsesSameOverlayBehavior(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	backgroundTop := strings.Split(m.renderMain(), "\n")[0]
	m.openCreate()

	modal := m.renderCreateModal()

	if top := strings.Split(modal, "\n")[0]; top != backgroundTop {
		t.Fatalf("create modal replaced background top row\n got: %q\nwant: %q", top, backgroundTop)
	}
	plain := ansi.Strip(modal)
	for _, want := range []string{"New task · Summary", "Description", "enter save", "esc cancel"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("create popup missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Save: enter | Cancel: esc") {
		t.Fatalf("create popup still uses legacy single-panel actions:\n%s", plain)
	}
}

func TestEditModalLoadsDescriptionAndUsesTwoPanelFocus(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	m.issues[0].Description = "Existing description\nWith details"
	m.openEdit()
	if m.createSummary.Value() != m.issues[0].Summary || m.createDescription.Value() != m.issues[0].Description {
		t.Fatalf("edit fields = %q / %q", m.createSummary.Value(), m.createDescription.Value())
	}

	m.updateCreate(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	if m.createFocus != 1 {
		t.Fatalf("tab focus = %d, want description", m.createFocus)
	}
	m.updateCreate(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if m.screen != screenCreate {
		t.Fatal("enter in description submitted instead of inserting a newline")
	}
	if modal := ansi.Strip(m.renderCreateModal()); !strings.Contains(modal, "ctrl+s save") {
		t.Fatalf("description focus lacks editor submit hint:\n%s", modal)
	}

	_, descriptionRect := m.createPopupRects()
	m.updateCreateMouse(tea.MouseClickMsg{X: descriptionRect.X + 1, Y: descriptionRect.Y + 1, Button: tea.MouseLeft})
	if m.createFocus != 1 {
		t.Fatalf("description click focus = %d", m.createFocus)
	}
}

func TestDetailsRenderDescription(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	m.issues[0].Description = "Acceptance criteria\nSecond paragraph"
	plain := ansi.Strip(strings.Join(m.detailsLines(35), "\n"))
	for _, want := range []string{"Description", "Acceptance criteria", "Second paragraph"} {
		if !strings.Contains(plain, want) {
			t.Errorf("details missing %q: %s", want, plain)
		}
	}
}

type deleteTestService struct {
	service.Service
	deleted string
	err     error
}

func (s *deleteTestService) DeleteTask(_ context.Context, key string) error {
	s.deleted = key
	return s.err
}

func TestDeleteTaskConfirmation(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	svc := &deleteTestService{}
	m.service = svc
	key := m.issues[0].Key
	backgroundTop := strings.Split(m.renderMain(), "\n")[0]

	m.openDelete()
	if m.screen != screenDelete || m.deletingTaskKey != key {
		t.Fatalf("delete popup state = %v / %q", m.screen, m.deletingTaskKey)
	}
	modal := ansi.Strip(m.renderDeleteModal())
	if top := strings.Split(m.renderDeleteModal(), "\n")[0]; top != backgroundTop {
		t.Fatalf("delete modal replaced background top row\n got: %q\nwant: %q", top, backgroundTop)
	}
	for _, want := range []string{"Delete task", key, "permanently", "enter confirm", "esc cancel"} {
		if !strings.Contains(modal, want) {
			t.Errorf("delete popup missing %q:\n%s", want, modal)
		}
	}

	m.updateDelete(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.screen != screenMain {
		t.Fatal("escape did not cancel deletion")
	}
	m.openDelete()
	cmd := m.updateDelete(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("confirm did not start deletion")
	}
	msg := cmd()
	if svc.deleted != key {
		t.Fatalf("deleted key = %q, want %q", svc.deleted, key)
	}
	m.Update(msg)
	if _, ok := m.issueByKey(key); ok || m.screen != screenMain {
		t.Fatalf("successful deletion retained issue or popup: screen=%v", m.screen)
	}
}

func TestDeleteTaskFailureKeepsConfirmationOpen(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	m.service = &deleteTestService{err: errors.New("permission denied")}
	m.openDelete()
	cmd := m.deleteTaskCmd()
	m.Update(cmd())
	if m.screen != screenDelete || m.loading || m.err == nil {
		t.Fatalf("failed delete state: screen=%v loading=%v err=%v", m.screen, m.loading, m.err)
	}
}

func TestReportModalUsesSameOverlayBehavior(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	backgroundTop := strings.Split(m.renderMain(), "\n")[0]
	m.screen = screenReport
	m.reportDraft.Subject = "Daily update"

	modal := m.renderReport()

	if top := strings.Split(modal, "\n")[0]; top != backgroundTop {
		t.Fatalf("report modal replaced background top row\n got: %q\nwant: %q", top, backgroundTop)
	}
	plain := ansi.Strip(modal)
	if !strings.Contains(plain, "Daily Report") || !strings.Contains(plain, "ctrl+s save  •  esc cancel") {
		t.Fatalf("report popup missing bottom-border actions:\n%s", plain)
	}
}

func TestKeybindingsCanOverlayPointsScreen(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	m.screen = screenPoints
	m.openKeybindingsModal()
	if m.modalParent != screenPoints || m.screen != screenHelp {
		t.Fatalf("modal parent/screen = %v/%v", m.modalParent, m.screen)
	}
	modal := ansi.Strip(m.renderKeybindingsModal())
	if !strings.Contains(modal, "Change: ←/→") || !strings.Contains(modal, "Keybindings") {
		t.Fatalf("keybindings did not overlay points screen:\n%s", modal)
	}
}

func TestKeybindingsModalNavigationAndClose(t *testing.T) {
	m := newMainTestModel(t, 80, 14)
	m.modalParent = screenMain
	m.screen = screenHelp

	m.updateKeybindingsModal(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.keybindingsViewport.Offset != 1 {
		t.Fatalf("down offset = %d, want 1", m.keybindingsViewport.Offset)
	}
	m.updateKeybindingsModal(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	lineCount, pageSize := m.keybindingsModalMetrics()
	if want := clampOffset(lineCount, lineCount, pageSize); m.keybindingsViewport.Offset != want {
		t.Fatalf("end offset = %d, want %d", m.keybindingsViewport.Offset, want)
	}
	m.updateKeybindingsModal(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.screen != screenMain {
		t.Fatalf("close returned to screen %v, want main", m.screen)
	}
}
