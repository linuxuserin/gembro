package main

import (
	"fmt"
	"log"
	neturl "net/url"
	"strconv"
	"strings"

	"git.sr.ht/~rafael/gembro/gemini"
	"git.sr.ht/~rafael/gembro/gopher"
	"git.sr.ht/~rafael/gembro/internal/history"
	"git.sr.ht/~rafael/gembro/text"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	buttonBack     = "Back"
	buttonFwd      = "Forward"
	buttonHome     = "Home"
	buttonBookmark = "Bookmark"
	buttonDownload = "Download"
	buttonGoto     = "Goto"
	buttonCloseTab = "Close Tab"
	buttonQuit     = "Quit"
	buttonHelp     = "Help"
	buttonEdit     = "Edit"
)

type Viewport struct {
	viewport       viewport.Model
	spinner        spinner.Model
	footer         Footer
	ready          bool
	loading        bool
	URL, MediaType string
	startScroll    int

	title     string
	links     text.Links
	lastEvent tea.MouseEventType
	history   *history.History
	digits    string
}

func NewViewport(startURL string, scrollPos int, h *history.History) Viewport {
	s := spinner.NewModel()
	s.Spinner = spinner.Points
	// footerLead := "Back (RMB) Forward (->) Home (h) Bookmark (b) Download (d) Close tab (q) Quit (ctrl+c) "
	return Viewport{
		URL:         startURL,
		startScroll: scrollPos,
		spinner:     s,
		history:     h,
		footer:      NewFooter(buttonBack, buttonFwd, buttonHome, buttonBookmark, buttonDownload, buttonHelp, buttonQuit),
	}
}

func (v Viewport) SetGoperContent(data []byte, url string, typ byte) Viewport {
	v.URL = url
	switch typ {
	case 'h':
		v.MediaType = "text/html"
	case 'I':
		v.MediaType = "image/jpeg"
	default:
		v.MediaType = "text/plain"
	}
	v.title = "Gopher"
	var content string
	content, v.links = gopher.ToANSI(data, typ)
	content = text.ApplyMargin(content, v.viewport.Width, gopher.TextWidth)
	v.viewport.SetContent(content)
	v.viewport.GotoTop()
	return v
}

func (v Viewport) SetGeminiContent(content, url, mediaType string, scrollPos int) Viewport {
	v.URL = url
	v.MediaType = mediaType
	u, _ := neturl.Parse(url)
	var s string

	switch mediaType := strings.Split(mediaType, ";")[0]; mediaType {
	case "text/gemini":
		s, v.links, v.title = gemini.ToANSI(content, v.viewport.Width, *u)
	default:
		if strings.HasPrefix(mediaType, "text/") {
			s = text.ApplyMargin(content, v.viewport.Width, gemini.TextWidth)
			v.links = text.Links{}
			v.title = url
		} else {
			s = fmt.Sprintf("Can't render content of this type: %s\n", mediaType)
		}
	}

	v.viewport.SetContent(s)
	v.viewport.SetYOffset(scrollPos)
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
				startURL = homeURL
			}
			hist, _ := v.history.Current()
			return v, fireEvent(LoadURLEvent{URL: startURL, ScrollPos: v.startScroll,
				AddHistory: hist != startURL})
		} else {
			v.viewport.Width = msg.Width
			v.viewport.Height = msg.Height - verticalMargins
		}
	case tea.MouseMsg:
		v, cmd = v.handleMouse(msg)
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "q":
			return v, v.handleButtonClick(buttonCloseTab)
		case "g":
			return v, v.handleButtonClick(buttonGoto)
		case "d":
			return v, v.handleButtonClick(buttonDownload)
		case "H":
			return v, v.handleButtonClick(buttonHome)
		case "b":
			return v, v.handleButtonClick(buttonBookmark)
		case "?":
			return v, v.handleButtonClick(buttonHelp)
		case "e":
			return v, v.handleButtonClick(buttonEdit)
		case "left", "h":
			return v, v.handleButtonClick(buttonBack)
		case "right", "l":
			return v, v.handleButtonClick(buttonFwd)
		case "esc":
			v.digits = ""
			return v, nil
		case "backspace":
			if len(v.digits) > 0 {
				v.digits = v.digits[0 : len(v.digits)-1]
				return v, nil
			}
		case "enter", "t":
			num, _ := strconv.Atoi(v.digits)
			link := v.links.Number(num)
			if link != nil {
				v.digits = ""
				if key == "t" {
					return v, fireEvent(OpenNewTabEvent{URL: link.URL, Switch: true})
				}
				return v, fireEvent(LoadURLEvent{URL: link.URL, AddHistory: true})
			}
		default:
			if "0" <= key && key <= "9" {
				v.digits += key
				return v, nil
			}
		}
	case ButtonClickEvent:
		return v, v.handleButtonClick(msg.Button)
	}

	if v.loading {
		v.spinner, cmd = v.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	v.footer, cmd = v.footer.Update(msg)
	cmds = append(cmds, cmd)
	v.viewport, cmd = v.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return v, tea.Batch(cmds...)
}

func (v Viewport) handleButtonClick(btn string) tea.Cmd {
	switch btn {
	case buttonBack:
		return fireEvent(GoBackEvent{})
	case buttonFwd:
		return fireEvent(GoForwardEvent{})
	case buttonHome:
		return fireEvent(LoadURLEvent{URL: homeURL, AddHistory: true})
	case buttonHelp:
		if v.URL == helpURL {
			return fireEvent(GoBackEvent{})
		}
		return fireEvent(LoadURLEvent{URL: helpURL, AddHistory: true})
	case buttonBookmark:
		return fireEvent(ToggleBookmarkEvent{URL: v.URL, Title: v.title})
	case buttonDownload:
		return fireEvent(ShowInputEvent{Message: "Download to",
			Value: suggestDownloadPath(v.title, v.URL, v.MediaType),
			Type:  inputDownloadSrc})
	case buttonGoto:
		var val string
		if cur, _ := v.history.Current(); cur != homeURL && cur != helpURL {
			val = cur
		}
		return fireEvent(ShowInputEvent{Message: "Go to", Type: inputNav, Payload: "", Value: val})
	case buttonCloseTab:
		return fireEvent(CloseCurrentTabEvent{})
	case buttonEdit:
		return fireEvent(EditSourceEvent{})
	case buttonQuit:
		return fireEvent(QuitEvent{})
	default:
		return nil
	}
}

func (v Viewport) View() string {
	if !v.ready {
		return "\n  Initalizing..."
	}

	var headerTail string
	if v.loading {
		headerTail = fmt.Sprintf(" :: %s", v.spinner.View())
	}
	if v.digits != "" {
		headerTail = fmt.Sprintf("%s :: %s", headerTail, v.digits)
	}
	header := fmt.Sprintf("%s%s ", v.URL, headerTail)
	gapSize := v.viewport.Width - text.RuneCount(header)
	header += strings.Repeat("???", gapSize)

	footer := fmt.Sprintf(" %3.f%%", v.viewport.ScrollPercent()*100)
	footerLead, fwidth := v.footer.View()
	gapSize = v.viewport.Width - text.RuneCount(footer) - fwidth
	if gapSize < 0 {
		gapSize = 0
	}
	footer = footerLead + strings.Repeat("???", gapSize) + footer

	return fmt.Sprintf("%s\n%s\n%s", header, v.viewport.View(), footer)
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
			if msg.Y == 1 {
				return viewport, fireEvent(ButtonClickEvent{buttonGoto})
			}
			if msg.Y >= viewport.viewport.Height+headerHeight {
				return viewport, fireEvent(FooterClickEvent{msg})
			}
			ypos := viewport.viewport.YOffset + msg.Y - headerHeight
			if link := viewport.links.LinkAt(ypos); link != nil {
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
