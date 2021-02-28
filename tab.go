package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"strings"
	"time"

	"git.sr.ht/~rafael/gembro/gemini"
	"git.sr.ht/~rafael/gembro/internal/bookmark"
	"git.sr.ht/~rafael/gembro/internal/history"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	messageLoadExternal
)

type tabID uint64

type Tab struct {
	id           tabID
	mode         mode
	input        Input
	message      Message
	viewport     Viewport
	client       *gemini.Client
	cancel       context.CancelFunc
	history      *history.History
	bookmarks    *bookmark.Store
	lastResponse GeminiResponse
}

func NewTab(client *gemini.Client, startURL string, bs *bookmark.Store, h *history.History, id tabID) Tab {
	ti := textinput.NewModel()
	ti.Placeholder = ""
	ti.CharLimit = 255
	ti.Width = 80
	if h == nil {
		h = &history.History{}
	}
	return Tab{
		id:        id,
		mode:      modePage,
		client:    client,
		history:   h,
		input:     NewInput(),
		viewport:  NewViewport(startURL, h),
		message:   Message{},
		bookmarks: bs,
	}
}

func (tab Tab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case error:
		log.Printf("%[1]T %[1]v", msg)
		var le LoadError
		if errors.As(msg, &le) {
			log.Print(le.Unwrap())
			tab.viewport.loading = false
		}
		if errors.Is(msg, context.Canceled) {
			return tab, nil
		}
		return tab.showMessage(msg.Error(), "", messagePlain, false)
	case MessageEvent:
		tab.mode = modePage
		switch msg.Type {
		case messageDelBookmark:
			if msg.Response {
				if err := tab.bookmarks.Remove(tab.lastResponse.URL); err != nil {
					log.Print(err)
				}
			}
		case messageLoadExternal:
			if msg.Response {
				if err := osOpenURL(msg.Payload); err != nil {
					log.Print(err)
				}
			}
		}
	case ShowMessageEvent:
		return tab.showMessage(msg.Message, msg.Payload, msg.Type, msg.WithConfirm)
	case CloseInputEvent:
		tab.mode = modePage
	case InputEvent:
		tab.mode = modePage
		switch msg.Type {
		case inputQuery:
			url := fmt.Sprintf("%s?%s", msg.Payload, neturl.QueryEscape(msg.Value))
			return tab.loadURL(url, true, 1)
		case inputNav:
			return tab.loadURL(msg.Value, true, 1)
		case inputBookmark:
			if err := tab.bookmarks.Add(msg.Payload, msg.Value); err != nil {
				log.Print(err)
			}
		case inputDownloadSrc:
			if err := tab.lastResponse.DownloadTo(msg.Value); err != nil {
				log.Print(err)
			}
		}
	case ShowInputEvent:
		return tab.showInput(msg.Message, msg.Value, msg.Payload, msg.Type)
	case LoadURLEvent:
		return tab.loadURL(msg.URL, msg.AddHistory, 1)
	case GoBackEvent:
		if url, ok := tab.history.Back(); ok {
			return tab.loadURL(url, false, 1)
		}
	case GoForwardEvent:
		if url, ok := tab.history.Forward(); ok {
			return tab.loadURL(url, false, 1)
		}
	case ToggleBookmarkEvent:
		if tab.bookmarks.Contains(msg.URL) {
			m := fmt.Sprintf("Remove %q from bookmarks?", msg.URL)
			return tab.showMessage(m, "", messageDelBookmark, true)
		}
		return tab.showInput("Name", msg.Title, msg.URL, inputBookmark)
	case GeminiResponse:
		return tab.handleResponse(msg)
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

	return tab, tea.Batch(cmds...)
}

func (tab Tab) View() string {
	switch tab.mode {
	case modeInput:
		return tab.input.View()
	case modeMessage:
		return tab.message.View()
	default:
		return tab.viewport.View()
	}
}

func (tab Tab) showMessage(msg, payload string, typ int, withConfirm bool) (Tab, tea.Cmd) {
	tab.message = Message{Message: msg,
		WithConfirm: withConfirm, Type: typ, Payload: payload}
	tab.mode = modeMessage
	return tab, nil
}

func (tab Tab) showInput(msg, val, payload string, typ int) (Tab, tea.Cmd) {
	tab.mode = modeInput
	tab.input = tab.input.Show(msg, val, payload, typ)
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
	tab     tabID
}

func (le LoadError) Unwrap() error {
	return le.err
}

func (le LoadError) Error() string {
	if le.err != nil {
		return fmt.Sprintf("%s: %s", le.message, le.err.Error())
	}
	return le.message
}

func (le LoadError) Tab() tabID {
	return le.tab
}

type GeminiResponse struct {
	*gemini.Response
	level int
	tab   tabID
}

func (gr GeminiResponse) Tab() tabID {
	return gr.tab
}

func (gi *GeminiResponse) DownloadTo(path string) error {
	err := os.WriteFile(path, gi.Source, 0644)
	if err != nil {
		return fmt.Errorf("could not complete download: %w", err)
	}
	return nil
}

func (tab Tab) handleResponse(resp GeminiResponse) (Tab, tea.Cmd) {
	tab.viewport.loading = false
	switch resp.Header.Status {
	case 1:
		return tab.showInput(resp.Header.Meta, "", resp.URL, inputQuery)
	case 3:
		if resp.level > 5 {
			return tab.showMessage("Too many redirects. Welcome to the Web from Hell.", "", messagePlain, false)
		}
		return tab.loadURL(resp.Header.Meta, false, resp.level+1)
	case 4, 5, 6:
		return tab.showMessage(fmt.Sprintf("Error: %s", resp.Header.Meta), "", messagePlain, false)
	case 2:
		body, err := resp.GetBody()
		if err != nil {
			log.Print(err)
			return tab, nil
		}
		tab.lastResponse = resp
		tab.viewport = tab.viewport.SetContent(body, resp.URL, resp.Header.Meta)
		return tab, nil
	default:
		log.Print(resp.Header)
		return tab, nil
	}
}

func (tab Tab) loadURL(url string, addHist bool, level int) (Tab, tea.Cmd) {
	if url != "home://" && !strings.HasPrefix(url, "gemini://") {
		tab.viewport.loading = false
		return tab.showMessage(fmt.Sprintf("Open %q externally?", url), url, messageLoadExternal, true)
	}
	if tab.cancel != nil {
		tab.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	tab.cancel = cancel
	tab.viewport.loading = true

	cmd := func() tea.Msg {
		defer cancel()

		if !strings.Contains(url, "://") {
			url = fmt.Sprintf("gemini://%s", url)
		}
		u, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		if url == "home://" {
			if addHist {
				tab.history.Add(url)
			}
			return GeminiResponse{Response: &gemini.Response{Body: tab.homeContent(),
				URL: url, Header: gemini.Header{Status: 2, Meta: "text/gemini"}}, level: level, tab: tab.id}
		}
		resp, err := tab.client.LoadURL(ctx, *u, true)
		if err := ctx.Err(); err != nil {
			return LoadError{err: err, message: "Could not load URL", tab: tab.id}
		}
		if err != nil {
			return LoadError{err: err, message: "Could not load URL", tab: tab.id}
		}
		if addHist {
			tab.history.Add(u.String())
		}
		return GeminiResponse{Response: resp, level: level, tab: tab.id}
	}
	return tab, tea.Batch(cmd, spinner.Tick)
}
