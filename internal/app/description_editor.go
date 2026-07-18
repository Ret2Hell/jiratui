package app

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
)

func textareaOffset(text string, line, column int) int {
	lines := strings.Split(text, "\n")
	line = min(max(0, line), len(lines)-1)
	offset := 0
	for index := range line {
		offset += len([]rune(lines[index])) + 1
	}
	return offset + min(max(0, column), len([]rune(lines[line])))
}

func setTextareaOffset(editor *textarea.Model, text string, offset int) {
	line, column := lineColumnAtOffset(text, offset)
	editor.MoveToBegin()
	for range line {
		editor.CursorDown()
	}
	editor.SetCursorColumn(column)
}

func lineColumnAtOffset(text string, offset int) (line, column int) {
	runes := []rune(text)
	offset = min(max(0, offset), len(runes))
	for _, character := range runes[:offset] {
		if character == '\n' {
			line++
			column = 0
		} else {
			column++
		}
	}
	return line, column
}

func insertDescriptionBlock(text string, offset int, block string) (string, int) {
	runes := []rune(text)
	offset = min(max(0, offset), len(runes))
	prefix := string(runes[:offset])
	suffix := string(runes[offset:])
	insertion := block
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		insertion = "\n" + insertion
	}
	if suffix != "" && !strings.HasPrefix(suffix, "\n") {
		insertion += "\n"
	}
	result := prefix + insertion + suffix
	return result, len([]rune(result)) - len(runes)
}
