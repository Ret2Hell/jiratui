package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMainFooterShowsOnlyPrimaryContextActions(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	footer := ansi.Strip(m.renderBindingFooter())
	for _, want := range []string{"New task: n", "Edit: e", "Story points: enter", "Keybindings: ?"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer %q missing %q", footer, want)
		}
	}
	for _, unwanted := range []string{"To Do", "Daily report", "Refresh", "Quit"} {
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
	for _, action := range []string{"create a new task", "move to In Progress", "move one page down", "save the report draft", "show all keybindings"} {
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

func TestCreateModalUsesSameOverlayBehavior(t *testing.T) {
	m := newMainTestModel(t, 120, 30)
	backgroundTop := strings.Split(m.renderMain(), "\n")[0]
	m.openCreate()

	modal := m.renderCreateModal()

	if top := strings.Split(modal, "\n")[0]; top != backgroundTop {
		t.Fatalf("create modal replaced background top row\n got: %q\nwant: %q", top, backgroundTop)
	}
	plain := ansi.Strip(modal)
	if !strings.Contains(plain, "New task") || !strings.Contains(plain, "Save: enter | Cancel: esc") {
		t.Fatalf("create popup missing shared modal chrome/actions:\n%s", plain)
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
	if !strings.Contains(plain, "Daily Report") || !strings.Contains(plain, "Save: ctrl+s | Cancel: esc") {
		t.Fatalf("report popup missing shared modal chrome/actions:\n%s", plain)
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
