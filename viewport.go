package main

import (
	"fmt"
	"log"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gembro/gemini/gemtext"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Viewport struct {
	viewport viewport.Model
	spinner  spinner.Model
	ready    bool
	loading  bool
	URL      string

	title     string
	links     []gemtext.LinkPos
	lastEvent tea.MouseEventType
}

func NewViewport(startURL string) Viewport {
	s := spinner.NewModel()
	s.Spinner = spinner.Points
	return Viewport{
		URL:     startURL,
		spinner: s,
	}
}

func (v Viewport) SetContent(content, url, mediaType string) Viewport {
	v.URL = url
	u, _ := neturl.Parse(url)
	var s string

	switch mediaType := strings.Split(mediaType, ";")[0]; mediaType {
	case "text/gemini":
		s, v.links, v.title = gemtext.ToANSI(content, v.viewport.Width, *u)
	case "text/plain", "text/html":
		s = gemtext.ApplyMargin(content, v.viewport.Width)
		v.links = nil
		v.title = url
	default:
		s = fmt.Sprintf("Can't render content of this type: %s\n", mediaType)
	}

	v.viewport.SetContent(s)
	v.viewport.GotoTop()
	return v
}

func (v Viewport) Update(msg tea.Msg) (Viewport, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		verticalMargins := headerHeight + footerHeight

		if !v.ready {
			v.viewport = viewport.Model{Width: msg.Width, Height: msg.Height - verticalMargins}
			v.viewport.YPosition = headerHeight
			v.viewport.HighPerformanceRendering = false
			v.viewport.SetContent("")
			v.ready = true
			startURL := v.URL
			if startURL == "" {
				startURL = "home://"
			}
			return v, fireEvent(LoadURLEvent{URL: startURL, AddHistory: true})
		} else {
			v.viewport.Width = msg.Width
			v.viewport.Height = msg.Height - verticalMargins
		}
	case tea.MouseMsg:
		v, cmd = v.handleMouse(msg)
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return v, fireEvent(CloseCurrentTabEvent{})
		case "g":
			return v, fireEvent(ShowInputEvent{Message: "Go to", Type: inputNav, Payload: ""})
		case "d":
			return v, fireEvent(ShowInputEvent{Message: "Download to", Value: suggestDownloadPath(v.title),
				Type: inputDownloadSrc})
		case "h":
			return v, fireEvent(LoadURLEvent{URL: "home://", AddHistory: true})
		case "b":
			return v, fireEvent(ToggleBookmarkEvent{URL: v.URL, Title: v.title})
		case "left":
			return v, fireEvent(GoBackEvent{})
		case "right":
			return v, fireEvent(GoForwardEvent{})
		}
	}

	if v.loading {
		v.spinner, cmd = v.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	v.viewport, cmd = v.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return v, tea.Batch(cmds...)
}

func (v Viewport) View() string {
	if !v.ready {
		return "\n  Initalizing..."
	}

	header := fmt.Sprintln(v.URL)
	if v.loading {
		header += fmt.Sprintf(" :: %s", v.spinner.View())
	}
	footer := fmt.Sprintf(" %3.f%%", v.viewport.ScrollPercent()*100)
	footerLead := "Back (RMB) Forward (->) Home (h) Bookmark (b) Download (d) Close tab (q) Quit (ctrl+c) "
	gapSize := v.viewport.Width - gemtext.RuneCount(footer) - gemtext.RuneCount(footerLead)
	if gapSize < 0 {
		gapSize = 0
	}
	footer = footerLead + strings.Repeat("─", gapSize) + footer

	return fmt.Sprintf("%s\n%s\n%s", header, v.viewport.View(), footer)
}

func (v Viewport) findLinkY(y int) *gemtext.LinkPos {
	for _, l := range v.links {
		if l.Y == y {
			return &l
		}
	}
	return nil
}

func (viewport Viewport) handleMouse(msg tea.MouseMsg) (Viewport, tea.Cmd) {
	log.Printf("Mouse event: %v", msg)
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg.Type {
	case tea.MouseLeft, tea.MouseMiddle, tea.MouseRight:
		viewport.lastEvent = msg.Type
	case tea.MouseRelease:
		switch viewport.lastEvent {
		case tea.MouseLeft, tea.MouseMiddle:
			if msg.Y == 0 {
				sel := msg.X / 4
				if viewport.lastEvent == tea.MouseMiddle {
					return viewport, fireEvent(CloseTabEvent{Tab: sel})
				} else {
					return viewport, fireEvent(SelectTabEvent{Tab: sel})
				}
			}
			ypos := viewport.viewport.YOffset + msg.Y - headerHeight
			if link := viewport.findLinkY(ypos); link != nil {
				if viewport.lastEvent == tea.MouseMiddle {
					cmd = fireEvent(OpenNewTabEvent{URL: link.URL})
					cmds = append(cmds, cmd)
				} else {
					return viewport, fireEvent(LoadURLEvent{URL: link.URL, AddHistory: true})
				}
			}
		case tea.MouseRight:
			return viewport, fireEvent(GoBackEvent{})
		}
	}
	return viewport, tea.Batch(cmds...)
}
