package app

import "slices"

type commandID string

const (
	cmdQuit     commandID = "quit"
	cmdHelp     commandID = "help"
	cmdRefresh  commandID = "refresh"
	cmdFilter   commandID = "filter"
	cmdNew      commandID = "new"
	cmdEdit     commandID = "edit"
	cmdPoints   commandID = "points"
	cmdTodo     commandID = "todo"
	cmdProgress commandID = "progress"
	cmdDone     commandID = "done"
	cmdReport   commandID = "report"
	cmdFocus    commandID = "focus"
	cmdUp       commandID = "up"
	cmdDown     commandID = "down"
	cmdPageUp   commandID = "page-up"
	cmdPageDown commandID = "page-down"
	cmdHome     commandID = "home"
	cmdEnd      commandID = "end"
	cmdSave     commandID = "save"
	cmdCancel   commandID = "cancel"
	cmdChange   commandID = "change"
	cmdSelect   commandID = "select"
)

type binding struct {
	Command            commandID
	Keys               []string
	Description, Short string
	Footer             bool
	Priority           int
}

func (m *Model) activeBindings() []binding {
	switch m.screen {
	case screenCreate:
		return []binding{
			{cmdSave, []string{"enter"}, "save task", "save", true, 10},
			{cmdFocus, []string{"tab", "shift+tab"}, "next/previous field", "fields", false, 5},
			{cmdCancel, []string{"esc"}, "cancel", "cancel", true, 10},
		}
	case screenPoints:
		return []binding{
			{cmdChange, []string{"left", "right", "up", "down", "h", "j", "k", "l"}, "change story points", "change", true, 10},
			{cmdSelect, []string{"0", "1", "2", "3", "4", "5", "6"}, "select story points", "select", true, 10},
			{cmdSave, []string{"enter"}, "save story points", "save", true, 10},
			{cmdCancel, []string{"esc"}, "cancel", "cancel", true, 10},
		}
	case screenReport:
		return []binding{{cmdSave, []string{"ctrl+s"}, "save report draft", "save", true, 10}, {cmdCancel, []string{"esc"}, "cancel", "cancel", true, 10}}
	case screenHelp:
		return []binding{{cmdCancel, []string{"esc", "?", "q"}, "close help", "close", true, 10}}
	case screenSetup:
		label := "continue"
		if m.setupStage == 1 {
			label = "save"
		}
		return []binding{{cmdFocus, []string{"tab", "shift+tab"}, "next/previous field", "fields", true, 10}, {cmdSave, []string{"enter"}, label, label, true, 10}, {cmdQuit, []string{"q"}, "quit", "quit", true, 1}}
	}
	b := []binding{
		{cmdNew, []string{"n"}, "new task", "new", true, 8},
		{cmdEdit, []string{"e", "shift+r"}, "edit selected task", "edit", true, 8},
		{cmdPoints, []string{"enter"}, "set story points", "points", true, 8},
		{cmdTodo, []string{"t"}, "move to To Do", "todo", true, 6},
		{cmdProgress, []string{"p", "i"}, "move to In Progress", "progress", true, 6},
		{cmdDone, []string{"d", "x"}, "move to Done", "done", true, 6},
		{cmdReport, []string{"m", "shift+m"}, "open daily report", "report", true, 5},
		{cmdFilter, []string{"/"}, "filter tickets", "filter", true, 7},
		{cmdRefresh, []string{"r"}, "refresh tickets", "refresh", true, 4},
		{cmdFocus, []string{"tab"}, "switch panel focus", "focus", true, 5},
		{cmdUp, []string{"up", "k"}, "move up", "up", false, 0}, {cmdDown, []string{"down", "j"}, "move down", "down", false, 0},
		{cmdPageUp, []string{"pgup"}, "page up", "page up", false, 0}, {cmdPageDown, []string{"pgdown"}, "page down", "page down", false, 0},
		{cmdHome, []string{"home", "g"}, "first item", "first", false, 0}, {cmdEnd, []string{"end", "shift+g"}, "last item", "last", false, 0},
		{cmdHelp, []string{"?"}, "show help", "help", true, 3}, {cmdQuit, []string{"q"}, "quit", "quit", true, 1},
	}
	if len(m.visibleIssues()) == 0 {
		b = slices.DeleteFunc(b, func(x binding) bool {
			return x.Command == cmdEdit || x.Command == cmdPoints || x.Command == cmdTodo || x.Command == cmdProgress || x.Command == cmdDone
		})
	}
	return b
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
		return b.Keys[0]
	}
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
