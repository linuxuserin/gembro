package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~rafael/gembro/gemini"
	"git.sr.ht/~rafael/gembro/gopher"
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
	messageForceCert
)

type tabID uint64

const (
	homeURL = "home://"
	helpURL = "help://"
)

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
	lastResponse ServerResponse
	specialPages map[string]func(Tab) string
}

func NewTab(client *gemini.Client, startURL string, scrollPos int, bs *bookmark.Store, h *history.History, id tabID) Tab {
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
		viewport:  NewViewport(startURL, scrollPos, h),
		message:   Message{},
		bookmarks: bs,
		specialPages: map[string]func(Tab) string{
			homeURL: homeContent,
			helpURL: helpContent,
		},
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
			if errors.Is(msg, gemini.CertChanged) {
				return tab.showMessage(fmt.Sprintf("The SSL certificate for %q has changed since last time.\n"+
					"Would you like to see the page anyway?", le.URL),
					le.URL, messageForceCert, true)
			}
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
				if err := tab.bookmarks.Remove(msg.Payload); err != nil {
					log.Print(err)
				}
			}
		case messageLoadExternal:
			if msg.Response {
				if err := osOpenURL(msg.Payload); err != nil {
					log.Print(err)
				}
			}
		case messageForceCert:
			if msg.Response {
				return tab.loadURL(msg.Payload, 0, true, 1, true)
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
			return tab.loadURL(url, 0, true, 1, false)
		case inputNav:
			return tab.loadURL(msg.Value, 0, true, 1, false)
		case inputBookmark:
			if err := tab.bookmarks.Add(msg.Payload, msg.Value); err != nil {
				log.Print(err)
			}
		case inputDownloadSrc:
			if err := DownloadTo(tab.lastResponse, msg.Value); err != nil {
				log.Print(err)
			}
		}
	case ShowInputEvent:
		return tab.showInput(msg.Message, msg.Value, msg.Payload, msg.Type)
	case LoadURLEvent:
		return tab.loadURL(msg.URL, msg.ScrollPos, msg.AddHistory, 1, false)
	case GoBackEvent:
		if url, pos, ok := tab.history.Back(); ok {
			return tab.loadURL(url, pos, false, 1, false)
		}
	case GoForwardEvent:
		if url, pos, ok := tab.history.Forward(); ok {
			return tab.loadURL(url, pos, false, 1, false)
		}
	case ToggleBookmarkEvent:
		if tab.bookmarks.Contains(msg.URL) {
			m := fmt.Sprintf("Remove %q from bookmarks?", msg.URL)
			return tab.showMessage(m, msg.URL, messageDelBookmark, true)
		}
		return tab.showInput("Name", msg.Title, msg.URL, inputBookmark)
	case EditSourceEvent:
		if err := editSource(tab.lastResponse.GetData()); err != nil {
			log.Print(err)
		}
		return tab, nil
	case ServerResponse:
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
	tab.message = NewMessage(msg, typ, withConfirm, payload)
	tab.mode = modeMessage
	return tab, nil
}

func (tab Tab) showInput(msg, val, payload string, typ int) (Tab, tea.Cmd) {
	tab.mode = modeInput
	tab.input = tab.input.Show(msg, val, payload, typ)
	return tab, textinput.Blink
}

func homeContent(tab Tab) string {
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

func helpContent(tab Tab) string {
	s := `
# Keys

Go back                 h
Go forward              l
Open link               type number + enter
Open link in tab        type number + t
Quit                    ctrl+c
Next tab                tab
Previous tab            shift+tab
Goto tab                alt+#
Close tab               q
Goto URL                g
Download page           d
Home                    H
Bookmark                b
View source (in gvim)   e
Scroll up               k
Scroll down             j
Scroll up (page)        Page up
Scroll down (page)      Page down


# Mouse

Open link               Left click
Open link in tab        Middle click
Close tab               Middle click (on tab)
Go back                 Right click
Scroll                  Mouse wheel
`
	return s
}

type LoadError struct {
	err     error
	message string
	tab     tabID
	URL     string
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

type ServerResponse interface {
	GetData() []byte
}

type GeminiResponse struct {
	*gemini.Response
	level     int
	scrollPos int
	tab       tabID
}

func (gr GeminiResponse) Tab() tabID {
	return gr.tab
}

func (gr GeminiResponse) GetData() []byte {
	return gr.Body
}

type GopherResponse struct {
	*gopher.Response
	tab tabID
}

func (gr GopherResponse) GetData() []byte {
	return gr.Data
}

func (gr GopherResponse) Tab() tabID {
	return gr.tab
}

func DownloadTo(resp ServerResponse, path string) error {
	err := os.WriteFile(path, resp.GetData(), 0644)
	if err != nil {
		return fmt.Errorf("could not complete download: %w", err)
	}
	return nil
}

func (tab Tab) handleResponse(resp ServerResponse) (Tab, tea.Cmd) {
	switch resp := resp.(type) {
	case GopherResponse:
		tab.viewport.loading = false
		tab.viewport = tab.viewport.SetGoperContent(resp.Data, resp.URL, resp.Type)
		tab.lastResponse = resp
		return tab, nil
	case GeminiResponse:
		tab.viewport.loading = false
		switch resp.Header.Status {
		case 1:
			return tab.showInput(resp.Header.Meta, "", resp.URL, inputQuery)
		case 3:
			if resp.level > 5 {
				return tab.showMessage("Too many redirects. Welcome to the Web from Hell.", "", messagePlain, false)
			}
			return tab.loadURL(resp.Header.Meta, resp.scrollPos, true, resp.level+1, false)
		case 4, 5, 6:
			return tab.showMessage(fmt.Sprintf("Error: %s", resp.Header.Meta), "", messagePlain, false)
		case 2:
			body, err := resp.GetBody()
			if err != nil {
				log.Print(err)
				return tab, nil
			}
			tab.lastResponse = resp
			tab.viewport = tab.viewport.SetGeminiContent(body, resp.URL, resp.Header.Meta, resp.scrollPos)
			return tab, nil
		default:
			log.Print(resp.Header)
			return tab, nil
		}
	}
	return tab, nil
}

func (tab Tab) loadURL(url string, scrollPos int, addHist bool, level int, skipVerify bool) (Tab, tea.Cmd) {
	if !strings.Contains(url, "://") {
		url = fmt.Sprintf("gemini://%s", url)
	}
	specialF, isSpecial := tab.specialPages[url]
	if !isSpecial && !strings.HasPrefix(url, "gemini://") && !strings.HasPrefix(url, "gopher://") {
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
		tab.history.UpdateScroll(tab.viewport.viewport.YOffset)
		defer cancel()

		if isSpecial {
			if addHist {
				tab.history.Add(url)
			}
			return GeminiResponse{Response: &gemini.Response{Body: []byte(specialF(tab)),
				URL: url, Header: gemini.Header{Status: 2, Meta: "text/gemini"}}, level: level, tab: tab.id}
		}

		u, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		switch u.Scheme {
		case "gopher":
			resp, err := gopher.LoadURL(ctx, *u)
			if err != nil {
				return LoadError{err: err, message: "could not load URL", tab: tab.id, URL: u.String()}
			}
			if addHist {
				tab.history.Add(u.String())
			}
			return GopherResponse{Response: resp, tab: tab.id}
		default: // gemini
			resp, err := tab.client.LoadURL(ctx, *u, skipVerify)
			if err := ctx.Err(); err != nil {
				return LoadError{err: err, message: "could not load URL", tab: tab.id, URL: u.String()}
			}
			if err != nil {
				return LoadError{err: err, message: "could not load URL", tab: tab.id, URL: u.String()}
			}
			if addHist && resp.Header.Status == 2 {
				tab.history.Add(u.String())
			}
			return GeminiResponse{Response: resp, level: level, tab: tab.id, scrollPos: scrollPos}
		}
	}
	return tab, tea.Batch(cmd, spinner.Tick)
}

func editSource(data []byte) error {
	cmd := exec.Command("gvim", "-")
	stderr, _ := cmd.StderrPipe()
	stdin, _ := cmd.StdinPipe()
	go stdin.Write(data)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, stderr)
		if buf.Len() > 0 {
			log.Print(buf.String())
		}
	}()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not run edit cmd: %w", err)
	}
	return nil
}
