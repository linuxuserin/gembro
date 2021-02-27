package gemtext

import (
	"fmt"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gembro/gemini"
	"github.com/muesli/termenv"
)

type Colo string

const (
	Ch1   Colo = "#FF0000" // red
	Ch2   Colo = "#FFFF00" // yellow
	Ch3   Colo = "#FF00FF" // fuchsia
	Clink Colo = "#6495ED" // cornflowerblue
	Ccode Colo = "#EEE8AA" // palegoldenrod
)

const textWidth = 80

var colors = termenv.ColorProfile()

// Color returns the input with the given ANSI color applied
// color can be a hex string e.g. #FF0000
func Color(input string, color Colo) string {
	return termenv.String(input).Foreground(colors.Color(string(color))).String()
}

type LinkPos struct {
	Y         int
	URL, Name string
}

// ToANSI convert Gemtext to text suitable for terminal output with colors
// It returns the converted text, a list of links with vertical positions, and the title of the page
// the title defaults to given baseURL when not found in the page
func ToANSI(data string, availableWidth int, baseURL neturl.URL) (
	content string, links []LinkPos, title string) {

	var s strings.Builder
	var mono bool
	ypos := 0
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "```") {
			mono = !mono
			continue
		}
		if !mono && strings.HasPrefix(line, "# ") {
			fmt.Fprintln(&s, Color(line[2:], Ch1))
			ypos++
			if title == "" {
				title = line[2:]
			}
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			fmt.Fprintln(&s, Color(line[3:], Ch2))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
			fmt.Fprintln(&s, Color(line[4:], Ch3))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "=>") {
			l, err := gemini.ParseLink(line)
			if err != nil {
				l = &gemini.Link{URL: "", Name: "Invalid link: " + line}
			}

			furl, _ := baseURL.Parse(l.URL)
			var extra string
			if furl.Scheme != "gemini" {
				extra = fmt.Sprintf(" (%s)", furl.Scheme)
			}
			fmt.Fprintf(&s, "> %s%s\n", Color(l.Name, Clink), extra)
			links = append(links, LinkPos{Y: ypos, URL: furl.String(), Name: l.Name})
			ypos++
			continue
		}
		if mono {
			fmt.Fprintln(&s, Color(line, Ccode))
			ypos++
			continue
		}

		sl := LineWrap(line, textWidth)
		for _, line := range strings.Split(sl, "\n") {
			fmt.Fprintln(&s, line)
			ypos++
		}
	}
	if title == "" {
		title = baseURL.String()
	}
	return ApplyMargin(s.String(), availableWidth), links, title
}

func ApplyMargin(input string, availableWidth int) string {
	margin := (availableWidth - textWidth) / 2
	indent := strings.Repeat(" ", margin)
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
