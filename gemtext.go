package main

import (
	"fmt"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/gemini"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"
)

const (
	red            = "#FF0000" // h1
	yellow         = "#FFFF00" // h2
	fuchsia        = "#FF00FF" // h3
	cornflowerblue = "#6495ED" // link
	palegoldenrod  = "#EEE8AA" // code
)

var colors = termenv.ColorProfile()

func color(input, color string) string {
	return termenv.String(input).Foreground(colors.Color(color)).String()
}

type linkPos struct {
	y         int
	url, name string
}

func parseContent(data string, width int, baseURL neturl.URL) (content string, links []linkPos, title string) {
	var s strings.Builder
	var mono bool
	var ypos int
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "```") {
			mono = !mono
			continue
		}
		if !mono && strings.HasPrefix(line, "# ") {
			fmt.Fprintln(&s, color(line[2:], red))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			fmt.Fprintln(&s, color(line[3:], yellow))
			ypos++
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
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
			fmt.Fprintf(&s, "> %s%s\n", color(l.Name, cornflowerblue), extra)
			links = append(links, linkPos{y: ypos, url: furl.String(), name: l.Name})
			ypos++
			continue
		}
		if mono {
			fmt.Fprintln(&s, color(line, palegoldenrod))
			ypos++
			continue
		}

		sl := wordwrap.String(line, width)
		fmt.Fprintln(&s, sl)
		ypos += strings.Count(sl, "\n") + 1
	}
	if title == "" {
		title = baseURL.String()
	}
	return s.String(), links, title
}
