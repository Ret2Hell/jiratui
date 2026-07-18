package app

import (
	"slices"
	"strings"
)

type commandID string

const (
	cmdQuit        commandID = "quit"
	cmdHelp        commandID = "help"
	cmdRefresh     commandID = "refresh"
	cmdFilter      commandID = "filter"
	cmdNew         commandID = "new"
	cmdEdit        commandID = "edit"
	cmdDelete      commandID = "delete"
	cmdPoints      commandID = "points"
	cmdTodo        commandID = "todo"
	cmdProgress    commandID = "progress"
	cmdDone        commandID = "done"
	cmdReport      commandID = "report"
	cmdFocus       commandID = "focus"
	cmdUp          commandID = "up"
	cmdDown        commandID = "down"
	cmdPageUp      commandID = "page-up"
	cmdPageDown    commandID = "page-down"
	cmdHome        commandID = "home"
	cmdEnd         commandID = "end"
	cmdSave        commandID = "save"
	cmdPasteImage  commandID = "paste-image"
	cmdRetrySave   commandID = "retry-save"
	cmdAbandonSave commandID = "abandon-save"
	cmdCancel      commandID = "cancel"
	cmdChange      commandID = "change"
	cmdSelect      commandID = "select"
	cmdClear       commandID = "clear"
)

type binding struct {
	Command            commandID
	Keys               []string
	Description, Short string
	Footer             bool
	Priority           int
}

type bindingGroup struct {
	Title    string
	Bindings []binding
}

func mainBindings() []binding {
	return []binding{
		{cmdNew, []string{"n"}, "create a new task", "Create", true, 8},
		{cmdEdit, []string{"e", "shift+r"}, "edit the selected task", "Edit", false, 8},
		{cmdDelete, []string{"shift+d"}, "delete the selected task", "Delete", false, 8},
		{cmdPoints, []string{"enter"}, "set story points", "Story points", true, 8},
		{cmdTodo, []string{"t"}, "move to To Do", "To Do", true, 6},
		{cmdProgress, []string{"p", "i"}, "move to In Progress", "In Progress", true, 6},
		{cmdDone, []string{"d", "x"}, "move to Done", "Done", true, 6},
		{cmdReport, []string{"m", "shift+m"}, "open daily report", "Daily report", false, 5},
		{cmdFilter, []string{"/"}, "filter tickets", "Filter", false, 7},
		{cmdRefresh, []string{"r"}, "refresh tickets", "Refresh", false, 4},
		{cmdRetrySave, []string{"ctrl+r"}, "retry the selected task save", "Retry save", true, 9},
		{cmdAbandonSave, []string{"shift+a"}, "abandon the selected task save", "Abandon save", true, 9},
		{cmdFocus, []string{"tab"}, "switch panel focus", "Switch panel", false, 5},
		{cmdUp, []string{"up", "k"}, "move up", "Up", false, 0},
		{cmdDown, []string{"down", "j"}, "move down", "Down", false, 0},
		{cmdPageUp, []string{"pgup"}, "move one page up", "Page up", false, 0},
		{cmdPageDown, []string{"pgdown"}, "move one page down", "Page down", false, 0},
		{cmdHome, []string{"home", "g"}, "go to the first item", "First", false, 0},
		{cmdEnd, []string{"end", "shift+g"}, "go to the last item", "Last", false, 0},
		{cmdHelp, []string{"?"}, "show all keybindings", "Keybindings", true, 3},
		{cmdQuit, []string{"q", "ctrl+c"}, "quit", "Quit", false, 1},
	}
}

func createBindings() []binding {
	return []binding{
		{cmdSave, []string{"enter", "ctrl+s"}, "create or save the task", "Save", true, 10},
		{cmdPasteImage, []string{"ctrl+o"}, "paste a clipboard image into the description", "Paste image", true, 8},
		{cmdFocus, []string{"tab", "shift+tab"}, "move between fields", "Fields", false, 5},
		{cmdCancel, []string{"esc"}, "close without saving", "Cancel", true, 10},
	}
}

func deleteBindings() []binding {
	return []binding{
		{cmdSave, []string{"enter"}, "confirm task deletion", "Delete", true, 10},
		{cmdCancel, []string{"esc"}, "cancel task deletion", "Cancel", true, 10},
	}
}

func pointsBindings() []binding {
	return []binding{
		{cmdChange, []string{"left", "right", "up", "down", "h", "j", "k", "l"}, "change story points", "Change", true, 10},
		{cmdSelect, []string{"0", "1", "2", "3", "4", "5", "6"}, "select a story-point value", "Select", true, 10},
		{cmdClear, []string{"u", "c"}, "set story points to unestimated", "Unestimated", true, 10},
		{cmdSave, []string{"enter"}, "save story points", "Save", true, 10},
		{cmdCancel, []string{"esc"}, "close without saving", "Cancel", true, 10},
	}
}

func reportBindings() []binding {
	return []binding{
		{cmdSave, []string{"ctrl+s"}, "save the report draft", "Save", true, 10},
		{cmdCancel, []string{"esc"}, "close without saving", "Cancel", true, 10},
	}
}

func setupBindings(saveLabel string) []binding {
	return []binding{
		{cmdFocus, []string{"tab", "shift+tab"}, "move between fields", "Fields", true, 10},
		{cmdSave, []string{"enter"}, saveLabel, saveLabel, true, 10},
		{cmdQuit, []string{"q"}, "quit", "Quit", true, 1},
	}
}

func keybindingsModalBindings() []binding {
	return []binding{
		{cmdUp, []string{"up", "k"}, "scroll up", "Scroll", false, 5},
		{cmdDown, []string{"down", "j"}, "scroll down", "Scroll", false, 5},
		{cmdPageUp, []string{"pgup"}, "page up", "Page up", false, 5},
		{cmdPageDown, []string{"pgdown"}, "page down", "Page down", false, 5},
		{cmdHome, []string{"home", "g"}, "go to top", "Top", false, 5},
		{cmdEnd, []string{"end", "shift+g"}, "go to bottom", "Bottom", false, 5},
		{cmdCancel, []string{"esc", "?", "q"}, "close keybindings", "Close", true, 10},
	}
}

func (m *Model) activeBindings() []binding {
	switch m.screen {
	case screenCreate:
		return m.activeCreateBindings()
	case screenDelete:
		return deleteBindings()
	case screenPoints:
		return pointsBindings()
	case screenReport:
		return reportBindings()
	case screenHelp:
		return keybindingsModalBindings()
	case screenSetup:
		label := "Continue"
		if m.setupStage == 1 {
			label = "Save"
		}
		return setupBindings(label)
	}

	bindings := mainBindings()
	if m.taskJournalsLoading {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool {
			return slices.Contains([]commandID{cmdNew, cmdEdit, cmdDelete, cmdPoints, cmdTodo, cmdProgress, cmdDone}, item.Command)
		})
	}
	_, hasTaskSave := m.selectedTaskSave()
	if !hasTaskSave {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool {
			return item.Command == cmdRetrySave || item.Command == cmdAbandonSave
		})
	} else {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool {
			return slices.Contains([]commandID{cmdEdit, cmdDelete, cmdPoints, cmdTodo, cmdProgress, cmdDone}, item.Command)
		})
	}
	if len(m.visibleIssues()) == 0 {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool {
			return slices.Contains([]commandID{cmdEdit, cmdDelete, cmdPoints, cmdTodo, cmdProgress, cmdDone}, item.Command)
		})
	}
	return bindings
}

func (m *Model) activeCreateBindings() []binding {
	bindings := createBindings()
	if m.createFocus == 1 {
		bindings[0].Keys = []string{"ctrl+s"}
	} else {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool { return item.Command == cmdPasteImage })
	}
	if !m.editingDescription.Editable || !m.imagePasteAvailable || m.attachmentMetaLoading {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool { return item.Command == cmdPasteImage })
	}
	if m.imagePastePending {
		bindings = slices.DeleteFunc(bindings, func(item binding) bool {
			return item.Command == cmdSave || item.Command == cmdPasteImage
		})
	}
	return bindings
}

func bindingsForCommands(bindings []binding, commands ...commandID) []binding {
	result := make([]binding, 0, len(commands))
	for _, command := range commands {
		if index := slices.IndexFunc(bindings, func(item binding) bool { return item.Command == command }); index >= 0 {
			result = append(result, bindings[index])
		}
	}
	return result
}

func bindingDisplayKey(b binding) string {
	switch b.Command {
	case cmdChange:
		return "←/→"
	case cmdSelect:
		return "0-6"
	default:
		if len(b.Keys) == 0 {
			return ""
		}
		return keyLabel(b.Keys[0])
	}
}

func keyLabel(key string) string {
	switch key {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	case "pgup":
		return "PgUp"
	case "pgdown":
		return "PgDn"
	case "shift+r":
		return "R"
	case "shift+m":
		return "M"
	case "shift+d":
		return "D"
	case "shift+g":
		return "G"
	case "shift+a":
		return "A"
	default:
		return key
	}
}

func bindingKeyList(b binding) string {
	labels := make([]string, len(b.Keys))
	for i, key := range b.Keys {
		labels[i] = keyLabel(key)
	}
	return strings.Join(labels, " / ")
}

func bindingForKey(bindings []binding, key string) (binding, bool) {
	var best binding
	found := false
	for _, b := range bindings {
		if slices.Contains(b.Keys, key) && (!found || b.Priority > best.Priority) {
			best, found = b, true
		}
	}
	return best, found
}
