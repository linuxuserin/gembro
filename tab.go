package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~rafael/gemini-browser/gemini"
	"git.sr.ht/~rafael/gemini-browser/internal/bookmark"
	"git.sr.ht/~rafael/gemini-browser/internal/history"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	inputNav = iota + 1
	inputQuery
	inputBookmark
	inputDownloadSrc
)

const (
	messagePlain = iota + 1
	messageDelBookmark
)

type Tab struct {
	mode         mode
	title        string
	startURL     string
	searchURL    string
	ready        bool
	loading      bool
	input        Input
	message      Message
	spinner      spinner.Model
	viewport     viewport.Model
	client       *gemini.Client
	cancel       context.CancelFunc
	history      *history.History
	bookmarks    *bookmark.Store
	links        []linkPos
	lastEvent    tea.MouseEventType
	lastResponse GeminiResponse
}

func NewTab(client *gemini.Client, startURL string, bs *bookmark.Store) Tab {
	ti := textinput.NewModel()
	ti.Placeholder = ""
	ti.CharLimit = 255
	ti.Width = 80

	s := spinner.NewModel()
	s.Spinner = spinner.Points

	return Tab{
		mode:      modePage,
		client:    client,
		title:     "Home",
		history:   &history.History{},
		input:     NewInput(),
		message:   Message{},
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
			log.Print(le.Unwrap())
			tab.loading = false
		}
		if errors.Is(msg, context.Canceled) {
			return tab, nil
		}
		return tab.showMessage(msg.Error(), messagePlain, false)
	case CloseMessageEvent:
		tab.mode = modePage
		switch msg.Type {
		case messageDelBookmark:
			if msg.Response {
				if err := tab.bookmarks.Remove(tab.lastResponse.URL); err != nil {
					log.Print(err)
				}
			}
		}
	case CloseInputEvent:
		tab.mode = modePage
	case InputEvent:
		tab.mode = modePage
		switch msg.Type {
		case inputQuery:
			url := fmt.Sprintf("%s?%s", tab.searchURL, neturl.QueryEscape(msg.Value))
			cmd, tab.loading, tab.cancel = tab.loadURL(url, true, 1)
			return tab, tea.Batch(cmd, spinner.Tick)
		case inputNav:
			cmd, tab.loading, tab.cancel = tab.loadURL(msg.Value, true, 1)
			return tab, tea.Batch(cmd, spinner.Tick)
		case inputBookmark:
			if err := tab.bookmarks.Add(tab.lastResponse.URL, msg.Value); err != nil {
				log.Print(err)
			}
		case inputDownloadSrc:
			if err := tab.lastResponse.DownloadTo(msg.Value); err != nil {
				log.Print(err)
			}
		}
	case tea.KeyMsg:
		log.Print(msg.String())
		skey := msg.String()
		switch tab.mode {
		case modePage:
			switch skey {
			case "q":
				return tab, fireEvent(CloseCurrentTabEvent{})
			case "g":
				return tab.showInput("Go to", "", inputNav)
			case "d":
				return tab.showInput("Download to", suggestDownloadPath(tab.title), inputDownloadSrc)
			case "h":
				cmd, tab.loading, tab.cancel = tab.loadURL("home://", true, 1)
				return tab, tea.Batch(cmd, spinner.Tick)
			case "b":
				if tab.bookmarks.Contains(tab.lastResponse.URL) {
					m := fmt.Sprintf("Remove %q from bookmarks?", tab.lastResponse.URL)
					return tab.showMessage(m, messageDelBookmark, true)
				}
				return tab.showInput("Name", tab.title, inputBookmark)
			case "left":
				if url, ok := tab.history.Back(); ok {
					cmd, tab.loading, tab.cancel = tab.loadURL(url, false, 1)
					return tab, tea.Batch(cmd, spinner.Tick)
				}
			case "right":
				if url, ok := tab.history.Forward(); ok {
					cmd, tab.loading, tab.cancel = tab.loadURL(url, false, 1)
					return tab, tea.Batch(cmd, spinner.Tick)
				}
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
			cmd, tab.loading, tab.cancel = tab.loadURL(startURL, true, 1)
			cmds = append(cmds, cmd, spinner.Tick)
		} else {
			tab.viewport.Width = msg.Width
			tab.viewport.Height = msg.Height - verticalMargins
		}
	case tea.MouseMsg:
		tab, cmd = tab.handleMouse(msg)
		cmds = append(cmds, cmd)
	case GeminiResponse:
		tab.loading = false
		switch msg.Header.Status {
		default:
			log.Print(msg.Header)
		case 1:
			tab.searchURL = msg.URL
			return tab.showInput(msg.Header.Meta, "", inputQuery)
		case 3:
			if msg.level > 5 {
				return tab.showMessage("Too many redirects. Welcome to the Web from Hell.", messagePlain, false)
			}
			cmd, tab.loading, tab.cancel = tab.loadURL(msg.Header.Meta, false, msg.level+1)
			return tab, tea.Batch(cmd, spinner.Tick)
		case 4, 5, 6:
			return tab.showMessage(fmt.Sprintf("Error: %s", msg.Header.Meta), messagePlain, false)
		case 2:
			body, err := msg.GetBody()
			if err != nil {
				log.Print(err)
				return tab, nil
			}
			tab.lastResponse = msg
			u, _ := neturl.Parse(msg.URL)
			var s string
			s, tab.links, tab.title = parseContent(body, tab.viewport.Width, *u)
			tab.viewport.SetContent(s)
			tab.viewport.GotoTop()
			return tab, nil
		}
	}

	switch tab.mode {
	case modeInput:
		tab.input, cmd = tab.input.Update(msg)
		cmds = append(cmds, cmd)
	case modeMessage:
		tab.message, cmd = tab.message.Update(msg)
		cmds = append(cmds, cmd)
	case modePage:
		tab.viewport, cmd = tab.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	if tab.loading {
		tab.spinner, cmd = tab.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
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
					cmd, tab.loading, tab.cancel = tab.loadURL(link.url, true, 1)
					cmds = append(cmds, cmd, spinner.Tick)
				}
			}
		case tea.MouseRight:
			if url, ok := tab.history.Back(); ok {
				cmd, tab.loading, tab.cancel = tab.loadURL(url, false, 1)
				return tab, tea.Batch(cmd, spinner.Tick)
			}
		}
	}
	return tab, tea.Batch(cmds...)
}

func (tab Tab) View() string {
	switch tab.mode {
	case modeInput:
		return tab.input.View()
	case modeMessage:
		return tab.message.View()
	default:
		if !tab.ready {
			return "\n  Initalizing..."
		}

		var header string
		if tab.lastResponse.Response != nil {
			header = tab.lastResponse.URL
		}
		if tab.loading {
			header += fmt.Sprintf(" :: %s", tab.spinner.View())
		}
		footer := fmt.Sprintf(" %3.f%%", tab.viewport.ScrollPercent()*100)
		footerLead := "Back (RMB) Forward (->) Close tab (q) Quit (ctrl+c) "
		gapSize := tab.viewport.Width - RuneCount(footer) - RuneCount(footerLead)
		footer = footerLead + strings.Repeat("─", gapSize) + footer

		return fmt.Sprintf("%s\n%s\n%s", header, tab.viewport.View(), footer)
	}
}

func (tab Tab) showMessage(msg string, typ int, withConfirm bool) (Tab, tea.Cmd) {
	tab.message = Message{Message: msg,
		WithConfirm: withConfirm, Type: typ}
	tab.mode = modeMessage
	return tab, nil
}

func (tab Tab) showInput(msg, val string, typ int) (Tab, tea.Cmd) {
	tab.mode = modeInput
	tab.input = tab.input.Show(msg, val, typ)
	return tab, textinput.Blink
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

type GeminiResponse struct {
	*gemini.Response
	level int
}

func suggestDownloadPath(name string) string {
	path, _ := os.UserHomeDir()
	downloadDir := filepath.Join(path, "Downloads")
	if _, err := os.Stat(downloadDir); err == nil { // Dir exists
		path = downloadDir
	}
	name = strings.NewReplacer(" ", "_", ".", "_").Replace(name)
	var extra string
	var count int
	for {
		newpath := filepath.Join(path, name+extra+".gmi")
		_, err := os.Stat(newpath)
		if os.IsNotExist(err) { // Not exists (or some other error)
			return newpath
		}
		count++
		if count > 100 { // Can't find available path, just suggest this one
			return newpath
		}
		extra = fmt.Sprintf("_%d", count)
	}
}

func (gi *GeminiResponse) DownloadTo(path string) error {
	err := os.WriteFile(path, gi.Source, 0644)
	if err != nil {
		return fmt.Errorf("could not complete download: %w", err)
	}
	return nil
}

func (tab Tab) loadURL(url string, addHist bool, level int) (func() tea.Msg, bool, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	loading := true

	return func() tea.Msg {
		// Cancel previous load
		if tab.cancel != nil {
			tab.cancel()
		}
		defer cancel()

		log.Print(url)
		u, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		if url == "home://" {
			if addHist {
				tab.history.Add(url)
			}
			return GeminiResponse{Response: &gemini.Response{Body: tab.homeContent(),
				URL: url, Header: gemini.Header{Status: 2, Meta: ""}}, level: level}
		}
		if u.Scheme != "gemini" {
			return &LoadError{err: nil, message: "Incorrect protocol"}
		}
		resp, err := tab.client.LoadURL(ctx, *u, true)
		if err := ctx.Err(); err != nil {
			return &LoadError{err: err, message: "Could not load URL"}
		}
		if err != nil {
			return &LoadError{err: err, message: "Could not load URL"}
		}
		if addHist {
			tab.history.Add(u.String())
		}
		return GeminiResponse{Response: resp, level: level}
	}, loading, cancel
}
