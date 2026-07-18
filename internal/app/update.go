package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/imageclip"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
)

// Update handles Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setComponentWidths()
		m.repairViewports()
	case cacheLoadedMsg:
		if msg.Err != nil {
			m.status = msg.Err.Error()
			break
		}
		if msg.OK {
			m.cacheRevision = max(m.cacheRevision, msg.State.Revision)
		}
		if msg.OK && len(m.issues) == 0 {
			selectedKey := m.selectedIssueKey()
			m.projectName = msg.State.ProjectName
			m.sprint = msg.State.Sprint
			m.issues = msg.State.Issues
			for _, journal := range m.discardedTaskSaves {
				m.discardTaskSaveProjection(journal)
			}
			m.reportDraft = msg.State.Draft
			if strings.TrimSpace(m.reportDraft.Body) == "" {
				m.refreshLocalDraft()
			}
			m.loading = false
			m.status = fmt.Sprintf("Loaded %d cached tickets", len(m.issues))
			m.recalcTotals()
			m.restoreSelection(selectedKey)
		}
	case cacheSavedMsg:
	case sprintLoadedMsg:
		selectedKey := m.selectedIssueKey()
		m.loading = false
		m.syncingSprint = false
		m.err = nil
		m.projectName = msg.Data.ProjectName
		m.sprint = msg.Data.Sprint
		m.mergeSprintData(msg.Data.Issues)
		m.restoreSelection(selectedKey)
		m.status = fmt.Sprintf("Synced %d tickets", len(m.issues))
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd(), m.generateReportCmd(false))
	case attachmentMetaLoadedMsg:
		m.attachmentMetaLoading = false
		if msg.Err != nil {
			m.imagePasteAvailable = false
			m.imagePasteUnavailableReason = "Jira attachment metadata is unavailable"
			break
		}
		m.imagePasteAvailable = msg.Meta.Enabled
		m.imagePasteUnavailableReason = "Jira attachments are disabled"
		if msg.Meta.Enabled {
			m.imagePasteUnavailableReason = ""
			m.imageUploadLimit = imageclip.MaxImageBytes
			if msg.Meta.UploadLimit > 0 {
				m.imageUploadLimit = min(m.imageUploadLimit, msg.Meta.UploadLimit)
			}
		}
	case taskJournalsLoadedMsg:
		m.taskJournalsLoading = false
		for _, journal := range msg.Journals {
			m.restoreTaskSave(journal)
		}
		for _, journal := range msg.Expired {
			m.discardTaskSaveProjection(journal)
		}
		m.discardedTaskSaves = append(m.discardedTaskSaves, msg.Expired...)
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Some Task Drafts could not be loaded: " + msg.Err.Error()
		} else if len(msg.Expired) > 0 {
			m.status = fmt.Sprintf("Expired %d Task Draft save(s); accepted Jira changes were retained", len(msg.Expired))
		} else if len(msg.Journals) > 0 {
			m.status = fmt.Sprintf("Recovered %d unfinished task save(s); select one to retry or abandon", len(msg.Journals))
		}
		m.recalcTotals()
		if len(msg.Expired) > 0 {
			cmds = append(cmds, m.saveCacheCmd(), m.loadSprintCmd())
		}
	case taskSaveFinishedMsg:
		delete(m.runningTaskSaves, msg.Journal.ID)
		previous := m.taskSaves[msg.Journal.ID]
		m.taskSaves[msg.Journal.ID] = msg.Journal
		projection := msg.Journal.Projection()
		oldKey := previous.IssueKey
		if oldKey == "" {
			oldKey = previous.TempKey
		}
		if oldKey == "" {
			oldKey = msg.Journal.TempKey
		}
		if oldKey != projection.Key {
			delete(m.pendingCreates, oldKey)
			m.replaceIssue(oldKey, projection)
		} else {
			m.replaceIssue(projection.Key, projection)
		}
		if msg.Journal.Complete() {
			delete(m.taskSaves, msg.Journal.ID)
			delete(m.pendingCreates, projection.Key)
			delete(m.pendingTaskUpdates, projection.Key)
			m.err = msg.Err
			if msg.Journal.Kind == tasksave.KindCreate {
				m.status = "Created " + projection.Key
			} else {
				m.status = "Updated " + projection.Key
			}
			cmds = append(cmds, m.saveCacheCmd(), m.loadSprintCmd())
		} else {
			if msg.Journal.Kind == tasksave.KindCreate {
				m.pendingCreates[projection.Key] = projection
			}
			m.err = msg.Err
			m.status = "Task Draft save paused for " + projection.Key
			if isPartialSave(msg.Journal) {
				m.status = "Partial Save for " + projection.Key
			}
			if msg.Err != nil {
				m.status += ": " + msg.Err.Error()
			}
			cmds = append(cmds, m.saveCacheCmd())
		}
		m.recalcTotals()
	case taskSaveAbandonedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Could not abandon Partial Save: " + msg.Err.Error()
			break
		}
		delete(m.taskSaves, msg.Journal.ID)
		delete(m.runningTaskSaves, msg.Journal.ID)
		projection := msg.Journal.Projection()
		delete(m.pendingCreates, projection.Key)
		delete(m.pendingCreates, msg.Journal.TempKey)
		delete(m.pendingTaskUpdates, msg.Journal.IssueKey)
		if msg.Journal.Kind == tasksave.KindCreate {
			m.removeIssue(projection.Key)
			if projection.Key != msg.Journal.TempKey {
				m.removeIssue(msg.Journal.TempKey)
			}
		} else {
			m.replaceIssue(msg.Journal.IssueKey, msg.Journal.Issue)
		}
		m.err = nil
		if msg.Expired {
			m.status = "Task save expired; accepted Jira changes were retained"
		} else {
			m.status = "Abandoned Task Draft save"
			if isPartialSave(msg.Journal) {
				m.status = "Abandoned Partial Save; accepted Jira changes were retained"
			}
		}
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd(), m.loadSprintCmd())
	case descriptionImagePastedMsg:
		if msg.SessionID != m.editorSessionID || m.screen != screenCreate {
			break
		}
		m.imagePastePending = false
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Image paste failed: " + msg.Err.Error()
			break
		}
		m.editingDescription.Images = append(m.editingDescription.Images, msg.Image)
		token := jira.ImageReferenceToken(msg.Image.ID, msg.Image.Filename)
		currentText := m.createDescription.Value()
		currentOffset := textareaOffset(currentText, m.createDescription.Line(), m.createDescription.Column())
		text, added := insertDescriptionBlock(currentText, msg.Offset, token)
		if msg.Offset <= currentOffset {
			currentOffset += added
		}
		m.createDescription.SetValue(text)
		setTextareaOffset(&m.createDescription, text, currentOffset)
		m.editingDescription.EditorText = text
		m.err = nil
		m.status = "Added " + msg.Image.Filename
	case taskDeletedMsg:
		m.loading = false
		m.err = nil
		m.removeIssue(msg.Key)
		delete(m.pendingTaskUpdates, msg.Key)
		delete(m.pendingPointOriginals, msg.Key)
		delete(m.pendingStatusOriginal, msg.Key)
		delete(m.localStatusChanges, msg.Key)
		m.deletingTaskKey = ""
		m.screen = screenMain
		m.status = "Deleted " + msg.Key
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd(), m.generateReportCmd(false))
	case taskDeleteFailedMsg:
		m.loading = false
		m.err = msg.Err
		m.status = "Delete failed: " + msg.Err.Error()
	case issueTransitionedMsg:
		if current, ok := m.issueByKey(msg.Key); !ok || statusEqual(current.Status, msg.Status) {
			delete(m.pendingStatusOriginal, msg.Key)
			m.updateIssueStatus(msg.Key, msg.Status)
			m.status = msg.Key + " → " + msg.Status.Name
		}
		m.screen = screenMain
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd(), m.generateReportCmd(false))
	case issueTransitionFailedMsg:
		if current, ok := m.issueByKey(msg.Key); ok && statusEqual(current.Status, msg.Target) {
			if original, ok := m.pendingStatusOriginal[msg.Key]; ok {
				m.updateIssueStatus(msg.Key, original)
				delete(m.pendingStatusOriginal, msg.Key)
			}
			delete(m.localStatusChanges, msg.Key)
		}
		m.err = msg.Err
		m.status = msg.Key + " status failed: " + msg.Err.Error()
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd())
	case pointsUpdatedMsg:
		if current, ok := m.issueByKey(msg.Key); !ok || pointsEqual(current.StoryPoints, msg.Points) {
			delete(m.pendingPointOriginals, msg.Key)
			m.updateIssuePoints(msg.Key, msg.Points)
			m.status = msg.Key + " points synced"
		}
		m.screen = screenMain
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case pointsUpdateFailedMsg:
		if current, ok := m.issueByKey(msg.Key); ok && pointsEqual(current.StoryPoints, msg.Points) {
			if original, ok := m.pendingPointOriginals[msg.Key]; ok {
				m.updateIssuePoints(msg.Key, cloneFloat(original))
				delete(m.pendingPointOriginals, msg.Key)
			}
		}
		m.err = msg.Err
		m.status = msg.Key + " points failed: " + msg.Err.Error()
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case reportGeneratedMsg:
		m.refreshingReport = false
		if msg.Version != m.reportVersion {
			break
		}
		m.reportDraft = msg.Draft
		openReport := msg.Open || m.openReportWhenReady
		m.openReportWhenReady = false
		if openReport {
			m.screen = screenReport
			m.reportEditor.SetValue(msg.Draft.Body)
			m.reportEditor.Focus()
		} else if m.screen != screenReport {
			m.status = "Report ready"
		}
		cmds = append(cmds, m.saveCacheCmd())
	case draftSavedMsg:
		m.loading = false
		m.screen = screenMain
		m.status = "Saved report to mail drafts"
	case jiraSetupSavedMsg:
		m.loading = false
		m.err = nil
		m.cfg = msg.Config
		m.setupStage = 1
		m.status = "Jira setup saved. Complete IONOS setup."
		m.focusSetup(m.setupStageStart())
	case setupSavedMsg:
		m.loading = true
		m.syncingSprint = true
		m.cfg = msg.Config
		m.service = msg.Service
		m.screen = screenMain
		m.taskJournalsLoading = true
		m.attachmentMetaLoading = true
		m.imagePasteAvailable = false
		m.status = "Setup saved"
		cmds = append(cmds, m.loadCacheCmd(), m.loadTaskJournalsCmd(), m.loadAttachmentMetaCmd(), m.loadSprintCmd())
	case errMsg:
		m.refreshingReport = false
		m.openReportWhenReady = false
		m.syncingSprint = false
		m.loading = false
		m.err = msg.Err
		m.status = msg.Err.Error()
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.Keystroke() == "ctrl+c" {
			return m, tea.Quit
		}
		if key.Keystroke() == "?" && m.screen == screenPoints {
			m.openKeybindingsModal()
			return m, tea.Batch(cmds...)
		}
		switch m.screen {
		case screenSetup:
			cmds = append(cmds, m.updateSetup(key))
		case screenMain:
			cmds = append(cmds, m.updateMain(key))
		case screenCreate:
			cmds = append(cmds, m.updateCreate(key, msg))
		case screenDelete:
			cmds = append(cmds, m.updateDelete(key))
		case screenPoints:
			cmds = append(cmds, m.updatePoints(key, msg))
		case screenReport:
			cmds = append(cmds, m.updateReport(key, msg))
		case screenHelp:
			cmds = append(cmds, m.updateKeybindingsModal(key))
		}
	}

	if _, ok := msg.(tea.PasteMsg); ok {
		cmds = append(cmds, m.updatePaste(msg))
	}

	if mouse, ok := msg.(tea.MouseClickMsg); ok {
		switch m.screen {
		case screenMain:
			m.updateMouse(mouse)
		case screenCreate:
			m.updateCreateMouse(mouse)
		}
	}
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		switch m.screen {
		case screenMain:
			m.updateMouseWheel(wheel)
		case screenHelp:
			m.scrollKeybindingsModal(wheel)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) updatePaste(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.screen {
	case screenSetup:
		m.setupInputs[m.setupFocus], cmd = m.setupInputs[m.setupFocus].Update(msg)
	case screenMain:
		if m.filtering {
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.selected = 0
			m.ticketViewport.Offset = 0
			m.detailsViewport.Offset = 0
			m.recalcTotals()
			m.refreshLocalDraft()
		}
	case screenCreate:
		if m.createFocus == 0 {
			m.createSummary, cmd = m.createSummary.Update(msg)
		} else if m.editingDescription.Editable {
			m.createDescription, cmd = m.createDescription.Update(msg)
			m.editingDescription.EditorText = m.createDescription.Value()
		}
	case screenPoints:
		// Story points are selected from fixed Fibonacci chips; paste is ignored.
	case screenReport:
		m.reportEditor, cmd = m.reportEditor.Update(msg)
	}
	return cmd
}

func (m *Model) updateSetup(key tea.KeyPressMsg) tea.Cmd {
	binding, ok := bindingForKey(m.activeBindings(), key.Keystroke())
	if ok {
		switch binding.Command {
		case cmdFocus:
			if key.Keystroke() == "tab" {
				m.focusSetup(m.setupFocus + 1)
				return nil
			}
			if m.setupFocus == m.setupStageStart() && m.setupStage > 0 {
				m.setupStage--
				m.focusSetup(m.setupStageEnd() - 1)
				return nil
			}
			m.focusSetup(m.setupFocus - 1)
			return nil
		case cmdSave:
			m.loading = true
			m.err = nil
			m.status = "Discovering Jira setup"
			if m.setupStage == 0 {
				return m.saveJiraSetupCmd()
			}
			return m.saveSetupCmd()
		case cmdQuit:
			return tea.Quit
		}
	}
	var cmd tea.Cmd
	m.setupInputs[m.setupFocus], cmd = m.setupInputs[m.setupFocus].Update(key)
	return cmd
}

func (m *Model) updateMain(key tea.KeyPressMsg) tea.Cmd {
	if m.filtering {
		switch key.Keystroke() {
		case "esc":
			m.filtering = false
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			m.selected, m.ticketViewport.Offset = 0, 0
			m.detailsViewport.Offset = 0
			m.recalcTotals()
			m.refreshLocalDraft()
			return nil
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(key)
		m.selected, m.ticketViewport.Offset = 0, 0
		m.detailsViewport.Offset = 0
		m.recalcTotals()
		m.refreshLocalDraft()
		return cmd
	}

	b, ok := bindingForKey(m.activeBindings(), key.Keystroke())
	if !ok {
		return nil
	}
	switch b.Command {
	case cmdQuit:
		return tea.Quit
	case cmdHelp:
		m.openKeybindingsModal()
	case cmdRefresh:
		m.syncingSprint = true
		m.loading = len(m.issues) == 0
		return m.loadSprintCmd()
	case cmdFocus:
		layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
		if !layout.TicketsOnly {
			if m.focus == focusTickets {
				m.focus = focusDetails
			} else {
				m.focus = focusTickets
			}
			m.repairViewports()
		}
	case cmdFilter:
		m.filtering = true
		m.filterInput.Focus()
	case cmdNew:
		m.openCreate()
	case cmdEdit:
		m.openEdit()
	case cmdDelete:
		m.openDelete()
	case cmdPoints:
		m.openPoints()
	case cmdTodo:
		return m.quickMoveCmd("new")
	case cmdProgress:
		return m.quickMoveCmd("indeterminate")
	case cmdDone:
		return m.quickMoveCmd("done")
	case cmdReport:
		return m.openReportCmd()
	case cmdRetrySave:
		return m.retryTaskSaveCmd()
	case cmdAbandonSave:
		return m.abandonTaskSaveCmd(false)
	case cmdUp:
		m.navigateFocused(-1, false)
	case cmdDown:
		m.navigateFocused(1, false)
	case cmdPageUp:
		m.navigateFocused(-1, true)
	case cmdPageDown:
		m.navigateFocused(1, true)
	case cmdHome:
		m.navigateBoundary(false)
	case cmdEnd:
		m.navigateBoundary(true)
	}
	return nil
}

func (m *Model) openKeybindingsModal() {
	m.modalParent = m.screen
	m.keybindingsViewport.Offset = 0
	m.keybindingsSelected = 0
	m.keybindingsFilter = ""
	m.keybindingsFiltering = false
	m.screen = screenHelp
}

func (m *Model) updateKeybindingsModal(key tea.KeyPressMsg) tea.Cmd {
	stroke := key.Keystroke()
	if m.keybindingsFiltering {
		switch stroke {
		case "esc":
			m.keybindingsFiltering = false
			m.keybindingsFilter = ""
		case "enter":
			m.keybindingsFiltering = false
			item, ok := m.selectedKeybinding()
			if ok && len(item.Keys) > 0 {
				return m.runKeybindingMenuItem(item)
			}
		case "backspace":
			runes := []rune(m.keybindingsFilter)
			if len(runes) > 0 {
				m.keybindingsFilter = string(runes[:len(runes)-1])
			}
		default:
			if key.Text != "" {
				m.keybindingsFilter += key.Text
			}
		}
		m.keybindingsSelected = 0
		m.keybindingsViewport.Offset = 0
		return nil
	}
	if stroke == "/" {
		m.keybindingsFiltering = true
		return nil
	}
	if stroke == "enter" {
		item, ok := m.selectedKeybinding()
		if !ok || len(item.Keys) == 0 {
			return nil
		}
		return m.runKeybindingMenuItem(item)
	}
	binding, ok := bindingForKey(m.activeBindings(), stroke)
	if !ok {
		return nil
	}
	count := m.selectableKeybindingCount()
	_, pageSize := m.keybindingsModalMetrics()
	switch binding.Command {
	case cmdCancel:
		m.screen = m.modalParent
		if m.screen == screenHelp {
			m.screen = screenMain
		}
	case cmdUp:
		m.keybindingsSelected = max(0, m.keybindingsSelected-1)
	case cmdDown:
		m.keybindingsSelected = min(max(0, count-1), m.keybindingsSelected+1)
	case cmdPageUp:
		m.keybindingsSelected = max(0, m.keybindingsSelected-pageSize)
	case cmdPageDown:
		m.keybindingsSelected = min(max(0, count-1), m.keybindingsSelected+pageSize)
	case cmdHome:
		m.keybindingsSelected = 0
	case cmdEnd:
		m.keybindingsSelected = max(0, count-1)
	}
	return nil
}

func (m *Model) runKeybindingMenuItem(item binding) tea.Cmd {
	parent := m.modalParent
	m.screen = parent
	key := keyPressForBinding(item.Keys[0])
	switch parent {
	case screenSetup:
		return m.updateSetup(key)
	case screenCreate:
		return m.updateCreate(key, key)
	case screenDelete:
		return m.updateDelete(key)
	case screenPoints:
		return m.updatePoints(key, key)
	case screenReport:
		return m.updateReport(key, key)
	default:
		return m.updateMain(key)
	}
}

func keyPressForBinding(value string) tea.KeyPressMsg {
	key := tea.Key{}
	switch value {
	case "enter":
		key.Code = tea.KeyEnter
	case "esc":
		key.Code = tea.KeyEscape
	case "tab":
		key.Code = tea.KeyTab
	case "shift+tab":
		key.Code, key.Mod = tea.KeyTab, tea.ModShift
	case "up":
		key.Code = tea.KeyUp
	case "down":
		key.Code = tea.KeyDown
	case "left":
		key.Code = tea.KeyLeft
	case "right":
		key.Code = tea.KeyRight
	case "pgup":
		key.Code = tea.KeyPgUp
	case "pgdown":
		key.Code = tea.KeyPgDown
	case "home":
		key.Code = tea.KeyHome
	case "end":
		key.Code = tea.KeyEnd
	default:
		parts := strings.Split(value, "+")
		character := parts[len(parts)-1]
		if len(parts) > 1 {
			switch parts[0] {
			case "ctrl":
				key.Mod = tea.ModCtrl
			case "shift":
				key.Mod = tea.ModShift
			}
		}
		if runes := []rune(character); len(runes) > 0 {
			key.Code = runes[0]
			key.Text = character
		}
	}
	return tea.KeyPressMsg(key)
}

func (m *Model) scrollKeybindingsModal(msg tea.MouseWheelMsg) {
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -3
	case tea.MouseWheelDown:
		delta = 3
	default:
		return
	}
	count := m.selectableKeybindingCount()
	m.keybindingsSelected = min(max(0, count-1), max(0, m.keybindingsSelected+delta))
}

func (m *Model) updateCreate(key tea.KeyPressMsg, msg tea.Msg) tea.Cmd {
	if binding, ok := bindingForKey(m.activeBindings(), key.Keystroke()); ok {
		switch binding.Command {
		case cmdCancel:
			m.editorSessionID++
			m.imagePastePending = false
			m.screen = screenMain
			return nil
		case cmdSave:
			if m.editingTaskKey != "" {
				return m.updateTaskCmd()
			}
			return m.createTaskCmd()
		case cmdPasteImage:
			return m.pasteDescriptionImageCmd()
		case cmdFocus:
			if key.Keystroke() == "tab" {
				m.focusCreate(m.createFocus + 1)
			} else {
				m.focusCreate(m.createFocus - 1)
			}
			return nil
		}
	}
	var cmd tea.Cmd
	if m.createFocus == 0 {
		m.createSummary, cmd = m.createSummary.Update(msg)
	} else {
		if m.editingDescription.Editable {
			m.createDescription, cmd = m.createDescription.Update(msg)
			m.editingDescription.EditorText = m.createDescription.Value()
		}
	}
	return cmd
}

func (m *Model) updateDelete(key tea.KeyPressMsg) tea.Cmd {
	binding, ok := bindingForKey(m.activeBindings(), key.Keystroke())
	if !ok {
		return nil
	}
	switch binding.Command {
	case cmdCancel:
		if !m.loading {
			m.deletingTaskKey = ""
			m.err = nil
			m.screen = screenMain
		}
	case cmdSave:
		return m.deleteTaskCmd()
	}
	return nil
}

func (m *Model) updatePoints(key tea.KeyPressMsg, _ tea.Msg) tea.Cmd {
	binding, ok := bindingForKey(m.activeBindings(), key.Keystroke())
	if !ok {
		return nil
	}
	switch binding.Command {
	case cmdCancel:
		m.cancelPointEdit()
		m.screen = screenMain
	case cmdSave:
		return m.updatePointsCmd()
	case cmdChange:
		switch key.Keystroke() {
		case "up", "left", "k", "h":
			m.movePointSelection(-1)
		default:
			m.movePointSelection(1)
		}
	case cmdSelect:
		m.pointSelected = int(key.Keystroke()[0]-'0') + 1
		m.applyPointSelection()
	case cmdClear:
		m.pointSelected = 0
		m.applyPointSelection()
	}
	return nil
}

func (m *Model) updateReport(key tea.KeyPressMsg, msg tea.Msg) tea.Cmd {
	if binding, ok := bindingForKey(m.activeBindings(), key.Keystroke()); ok {
		switch binding.Command {
		case cmdCancel:
			m.screen = screenMain
			return nil
		case cmdSave:
			m.reportDraft.Body = m.reportEditor.Value()
			m.loading = true
			return tea.Batch(m.saveCacheCmd(), m.saveDraftCmd())
		}
	}
	var cmd tea.Cmd
	m.reportEditor, cmd = m.reportEditor.Update(msg)
	return cmd
}

func (m *Model) updateCreateMouse(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	summaryRect, descriptionRect := m.createPopupRects()
	switch {
	case summaryRect.contains(msg.X, msg.Y):
		m.focusCreate(0)
	case descriptionRect.contains(msg.X, msg.Y):
		m.focusCreate(1)
	}
}

func (m *Model) updateMouse(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	if layout.Tickets.contains(msg.X, msg.Y) {
		m.focus = focusTickets
	}
	if !layout.TicketsOnly && layout.Details.contains(msg.X, msg.Y) {
		m.focus = focusDetails
	}
	visible := m.visibleIssues()
	for _, item := range visible {
		z := m.zones.Get(fmt.Sprintf("%sticket-%d", m.prefix, item.Index))
		if z != nil && z.InBounds(msg) {
			m.selectIssueBySourceIndex(item.Index)
			return
		}
	}
	m.repairViewports()
}

func (m *Model) selectIssueBySourceIndex(sourceIndex int) {
	visible := m.visibleIssues()
	row := slices.IndexFunc(visible, func(item indexedIssue) bool { return item.Index == sourceIndex })
	if row < 0 {
		return
	}
	m.selected = row
	m.detailsViewport.Offset = 0
	m.repairViewports()
}

func (m *Model) updateMouseWheel(msg tea.MouseWheelMsg) {
	layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -3
	case tea.MouseWheelDown:
		delta = 3
	default:
		return
	}
	if layout.Tickets.contains(msg.X, msg.Y) {
		m.focus = focusTickets
		m.moveSelection(delta)
	} else if !layout.TicketsOnly && layout.Details.contains(msg.X, msg.Y) {
		m.focus = focusDetails
		lines := m.detailsLines(max(1, layout.Details.Width-4))
		m.detailsViewport.Offset = clampOffset(m.detailsViewport.Offset+delta, len(lines), max(0, layout.Details.Height-2))
	}
}

func (m *Model) navigateFocused(delta int, page bool) {
	if m.focus == focusTickets {
		if page {
			m.selected = pageSelection(m.selected, delta, len(m.visibleIssues()), m.ticketPageSize())
			m.repairViewports()
		} else {
			m.moveSelection(delta)
		}
		m.detailsViewport.Offset = 0
		return
	}
	step := delta
	if page {
		step *= max(1, m.detailsPageSize())
	}
	detailsWidth, detailsPageSize := m.detailsPanelMetrics()
	m.detailsViewport.Offset = clampOffset(m.detailsViewport.Offset+step, len(m.detailsLines(detailsWidth)), detailsPageSize)
}

func (m *Model) navigateBoundary(last bool) {
	if m.focus == focusTickets {
		if last {
			m.selected = max(0, len(m.visibleIssues())-1)
		} else {
			m.selected = 0
		}
		m.detailsViewport.Offset = 0
		m.repairViewports()
		return
	}
	if last {
		detailsWidth, detailsPageSize := m.detailsPanelMetrics()
		m.detailsViewport.Offset = clampOffset(1<<30, len(m.detailsLines(detailsWidth)), detailsPageSize)
	} else {
		m.detailsViewport.Offset = 0
	}
}

func (m *Model) focusSetup(next int) {
	m.setupInputs[m.setupFocus].Blur()
	start := m.setupStageStart()
	end := m.setupStageEnd()
	if next < start {
		next = end - 1
	}
	if next >= end {
		next = start
	}
	m.setupFocus = next
	m.setupInputs[m.setupFocus].Focus()
}

func (m *Model) setupStageStart() int {
	if m.setupStage == 0 {
		return 0
	}
	return 4
}

func (m *Model) setupStageEnd() int {
	if m.setupStage == 0 {
		return 4
	}
	return len(m.setupInputs)
}

func (m *Model) focusCreate(next int) {
	m.createSummary.Blur()
	m.createDescription.Blur()
	m.createFocus = (next%2 + 2) % 2
	if m.createFocus == 0 {
		m.createSummary.Focus()
	} else {
		m.createDescription.Focus()
	}
}

func (m *Model) openCreate() {
	m.editorSessionID++
	m.imagePastePending = false
	m.editingTaskKey = ""
	m.editingTaskOriginal = ""
	m.editingTaskOriginalDescription = jira.Description{}
	m.editingDescription = jira.PlainDescription("")
	m.createSummary.SetValue("")
	m.createDescription.SetValue("")
	m.focusCreate(0)
	m.screen = screenCreate
}

func (m *Model) openEdit() {
	issue, ok := m.selectedIssue()
	if !ok || strings.HasPrefix(issue.Key, "NEW-") {
		return
	}
	m.editorSessionID++
	m.imagePastePending = false
	m.editingTaskKey = issue.Key
	m.editingTaskOriginal = issue.Summary
	description := issue.DescriptionContent
	if description.EditorText == "" && issue.Description != "" && len(description.RawADF) == 0 {
		description = jira.PlainDescription(issue.Description)
	}
	m.editingTaskOriginalDescription = description
	m.editingDescription = description
	m.createSummary.SetValue(issue.Summary)
	m.createDescription.SetValue(description.EditorText)
	m.focusCreate(0)
	m.screen = screenCreate
}

func (m *Model) openDelete() {
	issue, ok := m.selectedIssue()
	if !ok || strings.HasPrefix(issue.Key, "NEW-") {
		return
	}
	m.deletingTaskKey = issue.Key
	m.err = nil
	m.screen = screenDelete
}

func (m *Model) openPoints() {
	issue, ok := m.selectedIssue()
	if !ok {
		return
	}
	m.pointSelected = pointIndex(issue.StoryPoints)
	m.pointEditingKey = issue.Key
	m.pointOriginal = cloneFloat(issue.StoryPoints)
	m.status = "Choose story points for " + issue.Key
	m.screen = screenPoints
}

func (m *Model) movePointSelection(delta int) {
	m.pointSelected += delta
	if m.pointSelected < 0 {
		m.pointSelected = len(storyPointValues())
	}
	if m.pointSelected > len(storyPointValues()) {
		m.pointSelected = 0
	}
	m.applyPointSelection()
}

func (m *Model) applyPointSelection() {
	if m.pointEditingKey == "" {
		return
	}
	m.updateIssuePoints(m.pointEditingKey, selectedStoryPoints(m.pointSelected))
	m.recalcTotals()
}

func (m *Model) cancelPointEdit() {
	if m.pointEditingKey == "" {
		return
	}
	m.updateIssuePoints(m.pointEditingKey, cloneFloat(m.pointOriginal))
	m.recalcTotals()
	m.pointEditingKey = ""
	m.pointOriginal = nil
}

func (m *Model) mergeSprintData(remote []jira.Issue) {
	localByKey := make(map[string]jira.Issue, len(m.issues))
	for _, issue := range m.issues {
		localByKey[issue.Key] = issue
	}
	merged := make([]jira.Issue, 0, len(remote)+len(m.pendingCreates))
	for _, issue := range m.issues {
		if _, pending := m.pendingCreates[issue.Key]; pending {
			merged = append(merged, issue)
		}
	}
	for _, issue := range remote {
		if _, pending := m.pendingCreates[issue.Key]; pending {
			continue
		}
		if local, ok := localByKey[issue.Key]; ok {
			if _, pending := m.pendingTaskUpdates[issue.Key]; pending {
				issue.Summary = local.Summary
				issue.Description = local.Description
			}
			if _, pending := m.pendingStatusOriginal[issue.Key]; pending {
				issue.Status = local.Status
			}
			if _, pending := m.pendingPointOriginals[issue.Key]; pending {
				issue.StoryPoints = cloneFloat(local.StoryPoints)
			}
		}
		merged = append(merged, issue)
	}
	m.issues = merged
}

func (m *Model) restoreTaskSave(journal tasksave.Journal) {
	if journal.Complete() {
		return
	}
	m.taskSaves[journal.ID] = journal
	projection := journal.Projection()
	if journal.Kind == tasksave.KindCreate {
		key := journal.TempKey
		if journal.IssueCreated && journal.IssueKey != "" {
			key = journal.IssueKey
		}
		if key != journal.TempKey {
			m.removeIssue(journal.TempKey)
		}
		m.pendingCreates[key] = projection
		if sequence, err := strconv.Atoi(strings.TrimPrefix(journal.TempKey, "NEW-")); err == nil {
			m.tempIssueSeq = max(m.tempIssueSeq, sequence)
		}
		m.replaceIssue(key, projection)
		return
	}
	original := taskContent{Summary: journal.Issue.Summary, Description: journal.Issue.Description}
	desired := taskContent{Summary: projection.Summary, Description: projection.Description}
	m.pendingTaskUpdates[journal.IssueKey] = pendingTaskUpdate{Original: original, Desired: desired}
	m.replaceIssue(journal.IssueKey, projection)
}

func (m *Model) selectedTaskSave() (tasksave.Journal, bool) {
	issue, ok := m.selectedIssue()
	if !ok {
		return tasksave.Journal{}, false
	}
	for _, journal := range m.taskSaves {
		if issue.Key == journal.TempKey || issue.Key == journal.IssueKey || issue.Key == journal.Projection().Key {
			return journal, true
		}
	}
	return tasksave.Journal{}, false
}

func (m *Model) discardTaskSaveProjection(journal tasksave.Journal) {
	projection := journal.Projection()
	delete(m.pendingCreates, projection.Key)
	delete(m.pendingCreates, journal.TempKey)
	delete(m.pendingTaskUpdates, journal.IssueKey)
	if journal.Kind == tasksave.KindCreate {
		m.removeIssue(projection.Key)
		if projection.Key != journal.TempKey {
			m.removeIssue(journal.TempKey)
		}
		return
	}
	m.replaceIssue(journal.IssueKey, journal.Issue)
}

func (m *Model) replaceIssue(oldKey string, next jira.Issue) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == oldKey })
	if i < 0 {
		m.issues = append(m.issues, next)
		return
	}
	m.issues[i] = next
}

func (m *Model) removeIssue(key string) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == key })
	if i < 0 {
		return
	}
	m.issues = slices.Delete(m.issues, i, i+1)
	if m.selected >= len(m.visibleIssues()) {
		m.selected = max(0, len(m.visibleIssues())-1)
	}
	m.repairViewports()
}

func (m *Model) startStatusSync(key string, original jira.Status, next jira.Status) {
	m.err = nil
	if _, exists := m.pendingStatusOriginal[key]; !exists {
		m.pendingStatusOriginal[key] = original
	}
	m.trackLocalStatusChange(key, next)
	m.screen = screenMain
	m.status = key + " → " + next.Name + " queued"
	m.updateIssueStatus(key, next)
	m.recalcTotals()
	m.refreshLocalDraft()
}

func (m *Model) trackLocalStatusChange(key string, next jira.Status) {
	category := next.Category.Key
	if category == "" {
		category = jira.StatusCategoryForName(next.Name)
	}
	if category != "done" && category != "indeterminate" {
		delete(m.localStatusChanges, key)
		return
	}
	issue, _ := m.issueByKey(key)
	m.localStatusChanges[key] = jira.StatusChange{
		IssueKey:     key,
		IssueSummary: issue.Summary,
		ToStatus:     next.Name,
		ToCategory:   category,
		AuthorID:     m.cfg.Jira.AccountID,
		At:           time.Now(),
	}
}

func statusCategory(status jira.Status) string {
	if status.Category.Key != "" {
		return status.Category.Key
	}
	return jira.StatusCategoryForName(status.Name)
}

func optimisticStatus(target string) jira.Status {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "done":
		return jira.Status{Name: "Done", Category: jira.StatusCategory{Key: "done", Name: "Done"}}
	case "indeterminate", "progress", "in progress":
		return jira.Status{Name: "In Progress", Category: jira.StatusCategory{Key: "indeterminate", Name: "In Progress"}}
	default:
		return jira.Status{Name: "To Do", Category: jira.StatusCategory{Key: "new", Name: "To Do"}}
	}
}

func (m *Model) issueByKey(key string) (jira.Issue, bool) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == key })
	if i < 0 {
		return jira.Issue{}, false
	}
	return m.issues[i], true
}

func statusEqual(a jira.Status, b jira.Status) bool {
	if a.ID != "" && b.ID != "" && a.ID == b.ID {
		return true
	}
	return strings.EqualFold(a.Name, b.Name) && a.Category.Key == b.Category.Key
}

func pointsEqual(a *float64, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *Model) updateIssueStatus(key string, status jira.Status) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			m.issues[i].Status = status
			m.issues[i].Updated = time.Now()
			return
		}
	}
}

func (m *Model) updateIssueContent(key, summary, description string) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			m.issues[i].Summary = summary
			m.issues[i].Description = description
			m.issues[i].Updated = time.Now()
			return
		}
	}
}

func (m *Model) updateIssuePoints(key string, points *float64) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			m.issues[i].StoryPoints = points
			m.issues[i].Updated = time.Now()
			return
		}
	}
}

func (m *Model) parseJiraSetup() (config.Config, string, error) {
	cfg := m.cfg
	cfg.Jira.BaseURL = strings.TrimSpace(m.setupInputs[0].Value())
	cfg.Jira.Username = strings.TrimSpace(m.setupInputs[1].Value())
	jiraToken := strings.TrimSpace(m.setupInputs[2].Value())
	cfg.Jira.ProjectKey = strings.TrimSpace(m.setupInputs[3].Value())
	if cfg.Jira.BaseURL == "" || cfg.Jira.Username == "" || cfg.Jira.ProjectKey == "" {
		return cfg, "", fmt.Errorf("jira base URL, email, and project key are required")
	}
	return cfg, jiraToken, nil
}

func (m *Model) parseSetup() (config.Config, string, string, error) {
	cfg, jiraToken, err := m.parseJiraSetup()
	if err != nil {
		return cfg, "", "", err
	}
	cfg.Mail.IMAPHost = "imap.ionos.de"
	mailAddress := strings.TrimSpace(m.setupInputs[4].Value())
	cfg.Mail.From = mailAddress
	cfg.Mail.Username = mailAddress
	mailPassword := strings.TrimSpace(m.setupInputs[5].Value())
	cfg.Mail.To = splitCSV(m.setupInputs[6].Value())
	if cfg.Mail.From == "" || len(cfg.Mail.To) == 0 {
		return cfg, "", "", fmt.Errorf("mail sender and at least one report recipient are required")
	}
	return cfg, jiraToken, mailPassword, nil
}

func splitCSV(value string) []string {
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
