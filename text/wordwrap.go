package text

import (
	"bytes"
	"fmt"
	"strings"
)

const softHyphen = '\u00AD'

// TextWrap wraps text so that each line is no longer than maxlen by inserting newlines.
// Spaces and '-' will be used as natural breaking points, but if individual words are longer
// than maxlen a newline will be inserted to break them up.
func TextWrap(text string, maxlen int) string {
	var buf bytes.Buffer
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintln(&buf, lineWrap(line, maxlen))
	}
	return buf.String()
}

func lineWrap(line string, maxlen int) string {
	var buf, word bytes.Buffer
	var lineLen, wordLen int
	for _, r := range line {
		if r == '\r' {
			continue
		}
		if wordLen >= maxlen || r == '\n' {
			if buf.Len() > 0 {
				buf.WriteRune('\n')
			}
			buf.Write(word.Bytes())
			word.Reset()
			lineLen = 0
			wordLen = 0
		}
		word.WriteRune(r)
		wordLen++

		if r == ' ' || r == '-' || r == softHyphen {
			if lineLen+wordLen >= maxlen {
				buf.WriteString("\n")
				lineLen = 0
			}
			lineLen += wordLen
			buf.Write(word.Bytes())
			word.Reset()
			wordLen = 0
			continue
		}
	}
	if buf.Len() > 0 && (lineLen == 0 || lineLen+wordLen > maxlen) {
		buf.WriteRune('\n')
	}
	buf.Write(word.Bytes())
	return buf.String()
}

func RuneCount(s string) int {
	return len([]rune(s))
}

func ApplyMargin(input string, availableWidth, textWidth int) string {
	margin := (availableWidth - textWidth) / 2
	indent := strings.Repeat(" ", margin)
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
