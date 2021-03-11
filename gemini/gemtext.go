package gemini

import (
	"fmt"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gembro/text"
)

const TextWidth = 80

// ToANSI convert Gemtext to text suitable for terminal output with colors
// It returns the converted text, a list of links with vertical positions, and the title of the page
// the title defaults to given baseURL when not found in the page
func ToANSI(data string, availableWidth int, baseURL neturl.URL) (
	content string, links text.Links, title string) {

	var s strings.Builder
	var mono bool
	ypos := 0
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "```") {
			mono = !mono
			continue
		}
		if !mono && strings.HasPrefix(line, "# ") {
			fmt.Fprintln(&s, text.Color(line[2:], text.Ch1))
			ypos++
			if title == "" {
				title = line[2:]
			}
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			fmt.Fprintln(&s, text.Color(line[3:], text.Ch2))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
			fmt.Fprintln(&s, text.Color(line[4:], text.Ch3))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "=>") {
			l, err := ParseLink(line)
			if err != nil {
				l = &Link{URL: "", Name: "Invalid link: " + line}
			}

			furl, _ := baseURL.Parse(l.URL)
			var extra string
			if furl.Scheme != "gemini" {
				extra = fmt.Sprintf(" (%s)", furl.Scheme)
			}
			fmt.Fprintf(&s, "> %s%s\n", text.Color(l.Name, text.Clink), extra)
			links.Add(ypos, furl.String(), l.Name)
			ypos++
			continue
		}
		if mono {
			fmt.Fprintln(&s, text.Color(line, text.Ccode))
			ypos++
			continue
		}

		w := text.Wrap(line, TextWidth)
		fmt.Fprint(&s, w)
		ypos += strings.Count(w, "\n")
	}
	if title == "" {
		title = baseURL.String()
	}
	return text.ApplyMargin(s.String(), availableWidth, TextWidth), links, title
}
