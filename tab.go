package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/gemini"
	"git.sr.ht/~rafael/gemini-browser/internal/bookmark"
	"git.sr.ht/~rafael/gemini-browser/internal/history"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type Tab struct {
	mode       mode
	inputQuery string
	title      string
	Name       string
	startURL   string
	searchURL  string
	ready      bool
	loading    bool
	input      textinput.Model
	spinner    spinner.Model
	viewport   viewport.Model
	client     *gemini.Client
	cancel     context.CancelFunc
	history    *history.History
	bookmarks  *bookmark.Store
	links      []linkPos
	lastEvent  tea.MouseEventType
}

func NewTab(client *gemini.Client, startURL string, bs *bookmark.Store) Tab {
	ti := textinput.NewModel()
	ti.Placeholder = ""
	ti.CharLimit = 156
	ti.Width = 50

	s := spinner.NewModel()
	s.Spinner = spinner.Points

	return Tab{
		mode:      modePage,
		client:    client,
		title:     "Home",
		history:   &history.History{},
		input:     ti,
		spinner:   s,
		startURL:  startURL,
		bookmarks: bs,
	}
}

func (tab Tab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case error:
		log.Printf("%[1]T %[1]v", msg)
		var le *LoadError
		if errors.As(msg, &le) {
			tab.loading = false
		}
		if errors.Is(msg, context.Canceled) {
			return tab, nil
		}
		tab.mode = modeMessage
		tab.inputQuery = msg.Error()
		return tab, nil
	case tea.KeyMsg:
		log.Print(msg.String())
		skey := msg.String()
		switch tab.mode {
		case modeInput:
			switch skey {
			case "enter":
				tab.mode = modePage
				tab.input.Blur()
				tab.loading = true
				url := fmt.Sprintf("%s?%s", tab.searchURL, neturl.QueryEscape(tab.input.Value()))
				cmd, tab.loading, tab.cancel = tab.loadURL(url, true)
				return tab, tea.Batch(cmd, spinner.Tick)
			}
		case modeMessage:
			switch skey {
			case "enter":
				tab.mode = modePage
				tab.input.Blur()
				return tab, nil
			}
		case modeNavigate:
			switch skey {
			case "enter":
				tab.mode = modePage
				tab.input.Blur()
				cmd, tab.loading, tab.cancel = tab.loadURL(tab.input.Value(), true)
				return tab, tea.Batch(cmd, spinner.Tick)
			}
		case modePage:
			switch skey {
			case "g":
				tab.mode = modeNavigate
				tab.input.SetValue("")
				tab.input.Focus()
				return tab, nil
			case "left":
				if url, ok := tab.history.Back(); ok {
					cmd, tab.loading, tab.cancel = tab.loadURL(url, false)
					return tab, tea.Batch(cmd, spinner.Tick)
				}
			case "right":
				if url, ok := tab.history.Forward(); ok {
					cmd, tab.loading, tab.cancel = tab.loadURL(url, false)
					return tab, tea.Batch(cmd, spinner.Tick)
				}
			case "ctrl+c", "q":
				return tab, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		verticalMargins := headerHeight + footerHeight

		if !tab.ready {
			tab.viewport = viewport.Model{Width: msg.Width, Height: msg.Height - verticalMargins}
			tab.viewport.YPosition = headerHeight
			tab.viewport.HighPerformanceRendering = false
			tab.viewport.SetContent("")
			tab.ready = true
			startURL := tab.startURL
			if startURL == "" {
				startURL = "home://"
			}
			cmd, tab.loading, tab.cancel = tab.loadURL(startURL, true)
			cmds = append(cmds, cmd, spinner.Tick)
		} else {
			tab.viewport.Width = msg.Width
			tab.viewport.Height = msg.Height - verticalMargins
		}
	case tea.MouseMsg:
		return tab.handleMouse(msg)
	case *gemini.Response:
		tab.loading = false
		switch msg.Header.Status {
		default:
			log.Print(msg.Header)
		case 1:
			tab.mode = modeInput
			tab.searchURL = msg.URL
			tab.inputQuery = msg.Header.Meta
			tab.input.SetValue("")
			tab.input.Focus()
		case 4, 5, 6:
			tab.mode = modeMessage
			tab.inputQuery = fmt.Sprintf("Error: %s", msg.Header.Meta)
			tab.input.SetValue("")
			tab.input.Focus()
		case 2:
			body, err := msg.GetBody()
			if err != nil {
				log.Print(err)
				return tab, nil
			}
			u, _ := neturl.Parse(msg.URL)
			var s string
			s, tab.links, tab.title = parseContent(body, tab.viewport.Width, *u)
			tab.viewport.SetContent(s)
			tab.viewport.GotoTop()
			return tab, nil
		}
	}

	if tab.loading {
		tab.spinner, cmd = tab.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	tab.input, cmd = tab.input.Update(msg)
	cmds = append(cmds, cmd)
	tab.viewport, _ = tab.viewport.Update(msg)
	return tab, tea.Batch(cmds...)
}

func (tab Tab) findLinkY(y int) *linkPos {
	for _, l := range tab.links {
		if l.y == y {
			return &l
		}
	}
	return nil
}

func (tab Tab) handleMouse(msg tea.MouseMsg) (Tab, tea.Cmd) {
	log.Printf("Mouse event: %v", msg)
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg.Type {
	case tea.MouseLeft, tea.MouseMiddle, tea.MouseRight:
		tab.lastEvent = msg.Type
	case tea.MouseRelease:
		switch tab.lastEvent {
		case tea.MouseLeft, tea.MouseMiddle:
			if msg.Y == 0 {
				sel := msg.X / 4
				if tab.lastEvent == tea.MouseMiddle {
					return tab, fireEvent(CloseTabEvent{Tab: sel})
				} else {
					return tab, fireEvent(SelectTabEvent{Tab: sel})
				}
			}
			ypos := tab.viewport.YOffset + msg.Y - headerHeight
			if link := tab.findLinkY(ypos); link != nil {
				if tab.lastEvent == tea.MouseMiddle {
					cmd = fireEvent(OpenNewTabEvent{URL: link.url})
					cmds = append(cmds, cmd)
				} else {
					cmd, tab.loading, tab.cancel = tab.loadURL(link.url, true)
					cmds = append(cmds, cmd, spinner.Tick)
				}
			}
		case tea.MouseRight:
			if url, ok := tab.history.Back(); ok {
				cmd, tab.loading, tab.cancel = tab.loadURL(url, false)
				return tab, tea.Batch(cmd, spinner.Tick)
			}
		}
	}
	return tab, tea.Batch(cmds...)
}

func (tab Tab) View() string {
	switch tab.mode {
	case modeInput:
		return fmt.Sprintf("%s %s\n\nPress ENTER to continue", tab.inputQuery, tab.input.View())
	case modeMessage:
		return fmt.Sprintf("%s\n\nPress ENTER to continue", tab.inputQuery)
	case modeNavigate:
		return fmt.Sprintf("Goto %s\n\nPress ENTER to continue", tab.input.View())
	default:
		if !tab.ready {
			return "\n  Initalizing..."
		}

		header := tab.title
		if tab.loading {
			header += fmt.Sprintf(" :: %s", tab.spinner.View())
		}
		footer := fmt.Sprintf(" %3.f%%", tab.viewport.ScrollPercent()*100)
		footerLead := "Back (RMB) Forward (->) Close tab (q) "
		gapSize := tab.viewport.Width - runewidth.StringWidth(footer) - runewidth.StringWidth(footerLead)
		footer = footerLead + strings.Repeat("─", gapSize) + footer

		return fmt.Sprintf("%s\n%s\n%s", header, tab.viewport.View(), footer)
	}
}

func (tab Tab) homeContent() string {
	var buf strings.Builder
	fmt.Fprint(&buf, "# Home\n\n")
	for _, bookmark := range builtinBookmarks {
		fmt.Fprintf(&buf, "=> %s %s\n", bookmark.URL, bookmark.Name)
	}
	fmt.Fprintln(&buf)
	bookmarks := tab.bookmarks.All()
	for _, bookmark := range bookmarks {
		fmt.Fprintf(&buf, "=> %s %s\n", bookmark.URL, bookmark.Name)
	}
	return buf.String()
}

type LoadError struct {
	err     error
	message string
}

func (le *LoadError) Unwrap() error {
	return le.err
}

func (le *LoadError) Error() string {
	if le.err != nil {
		return fmt.Sprintf("%s: %s", le.message, le.err.Error())
	}
	return le.message
}

func (tab Tab) loadURL(url string, addHist bool) (func() tea.Msg, bool, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	loading := true

	return func() tea.Msg {
		log.Print(url)
		u, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		if url == "home://" {
			if addHist {
				tab.history.Add(url)
			}
			return &gemini.Response{Body: tab.homeContent(), URL: url, Header: gemini.Header{Status: 2, Meta: ""}}
		}
		if u.Scheme != "gemini" {
			return &LoadError{err: nil, message: "Incorrect protocol"}
		}
		if tab.cancel != nil {
			tab.cancel()
		}
		resp, err := tab.client.LoadURL(ctx, *u, true)
		if err != nil {
			return &LoadError{err: err, message: "Could not load URL"}
		}
		if addHist {
			tab.history.Add(u.String())
		}
		return resp
	}, loading, cancel
}
