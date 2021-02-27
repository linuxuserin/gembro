package gemtext

import (
	"fmt"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gembro/gemini"
	"github.com/muesli/termenv"
)

const (
	red            = "#FF0000" // h1
	yellow         = "#FFFF00" // h2
	fuchsia        = "#FF00FF" // h3
	cornflowerblue = "#6495ED" // link
	palegoldenrod  = "#EEE8AA" // code
)

const textWidth = 80

var colors = termenv.ColorProfile()

func color(input, color string) string {
	return termenv.String(input).Foreground(colors.Color(color)).String()
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
	margin := (availableWidth - textWidth) / 2
	indent := strings.Repeat(" ", margin)
	fmt.Fprintln(&s)
	ypos := 1
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "```") {
			mono = !mono
			continue
		}
		if !mono && strings.HasPrefix(line, "# ") {
			fmt.Fprint(&s, indent)
			fmt.Fprintln(&s, color(line[2:], red))
			ypos++
			if title == "" {
				title = line[2:]
			}
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			fmt.Fprint(&s, indent)
			fmt.Fprintln(&s, color(line[3:], yellow))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
			fmt.Fprint(&s, indent)
			fmt.Fprintln(&s, color(line[4:], fuchsia))
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
			fmt.Fprint(&s, indent)
			fmt.Fprintf(&s, "> %s%s\n", color(l.Name, cornflowerblue), extra)
			links = append(links, LinkPos{Y: ypos, URL: furl.String(), Name: l.Name})
			ypos++
			continue
		}
		if mono {
			fmt.Fprint(&s, indent)
			fmt.Fprintln(&s, color(line, palegoldenrod))
			ypos++
			continue
		}

		sl := LineWrap(line, textWidth)
		for _, line := range strings.Split(sl, "\n") {
			fmt.Fprint(&s, indent)
			fmt.Fprintln(&s, line)
			ypos++
		}
	}
	if title == "" {
		title = baseURL.String()
	}
	return s.String(), links, title
}
