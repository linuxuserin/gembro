package gemtext

import (
	"bytes"
)

const softHyphen = '\u00AD'

// LineWrap wraps text so that each line is no longer than maxlen by inserting newlines.
// Spaces and '-' will be used as natural breaking points, but if individual words are longer
// than maxlen a newline will be inserted to break them up.
func LineWrap(line string, maxlen int) string {
	var buf, word bytes.Buffer
	var lineLen, wordLen int
	for _, r := range line {
		if wordLen >= maxlen {
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
