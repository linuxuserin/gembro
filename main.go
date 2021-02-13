package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"
)

type model struct {
	url       string
	data      []string
	textInput textinput.Model
	viewport  viewport.Model
	cursor    int              // which to-do list item our cursor is pointing at
	selected  map[int]struct{} // which to-do items are selected
	err       error
	links     []LinkPos
	startURL  string
	ready     bool
	history   *History
}

func (m model) Init() tea.Cmd {
	if m.startURL != "" {
		return teaLoadURL(m.startURL, true)
	}
	return textinput.Blink
}

type errMsg error

type TeaResponse struct {
	*Response
	AddHist bool
}

func teaLoadURL(surl string, addHist bool) func() tea.Msg {
	return func() tea.Msg {
		u, err := url.Parse(surl)
		if err != nil {
			return errMsg(fmt.Errorf("invalid URL: %s", err))
		}
		resp, err := loadURL(*u)
		if err != nil {
			return errMsg(fmt.Errorf("could not load URL: %s", err))
		}
		return TeaResponse{resp, addHist}
	}
}

type LinkPos struct {
	link *Link
	ypos int
}

func parseContent(content []string, cursor int) (string, []LinkPos) {
	var linkIDX int
	var s strings.Builder
	var links []LinkPos
	var ypos int
	for _, line := range content {
		if strings.HasPrefix(line, "=>") {
			l, err := ParseLink(line)
			if err != nil {
				l = &Link{URL: "", Name: "Invalid link: " + line}
			}
			links = append(links, LinkPos{l, ypos})

			linkIDX++
			if linkIDX == cursor {
				s.WriteString(termenv.String(fmt.Sprintf("> %s (%s)", l.Name, l.URL)).Reverse().String())
			} else {
				s.WriteString(fmt.Sprintf("> %s", l.Name))
			}
			s.WriteString("\n")
			ypos++
			continue
		}
		sl := wordwrap.String(line, 100)
		s.WriteString(sl)
		s.WriteString("\n")
		ypos += strings.Count(sl, "\n") + 1
	}
	return s.String(), links
}

const (
	headerHeight               = 3
	footerHeight               = 1
	useHighPerformanceRenderer = false
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	updateContent := func() {
		if m.cursor == 0 {
			m.textInput.Focus()
		} else {
			m.textInput.Blur()
		}
		content, links := parseContent(m.data, m.cursor)
		m.links = links
		m.viewport.SetContent(content)
		if m.cursor != 0 {
			ypos := links[m.cursor-1].ypos
			if m.viewport.YOffset > ypos {
				m.viewport.YOffset = ypos
			}
			if m.viewport.YOffset+m.viewport.Height <= ypos {
				m.viewport.YOffset = ypos - m.viewport.Height + 1
			}
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyShiftTab:
			if m.cursor > 0 {
				m.cursor--
				updateContent()
			}
			return m, nil
		case tea.KeyTab:
			m.cursor = (m.cursor + 1) % (len(m.links) + 1)
			updateContent()
			return m, nil
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.textInput.Focused() {
				m.textInput.Blur()
			} else {
				m.textInput.Focus()
			}
			return m, nil
		case tea.KeyEnter:
			m.err = nil
			if m.cursor == 0 {
				return m, teaLoadURL(m.textInput.Value(), true)
			}
			surl := m.links[m.cursor-1].link.FullURL(m.url)
			return m, teaLoadURL(surl, true)
		case tea.KeyLeft:
			if prev, ok := m.history.Back(); ok {
				return m, teaLoadURL(prev, false)
			}
			return m, nil
		case tea.KeyRight:
			if next, ok := m.history.Forward(); ok {
				return m, teaLoadURL(next, false)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		verticalMargins := headerHeight + footerHeight

		if !m.ready {
			// Since this program is using the full size of the viewport we need
			// to wait until we've received the window dimensions before we
			// can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.Model{Width: msg.Width, Height: msg.Height - verticalMargins}
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = useHighPerformanceRenderer
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

		if useHighPerformanceRenderer {
			// Render (or re-render) the whole viewport. Necessary both to
			// initialize the viewport and when the window is resized.
			//
			// This is needed for high-performance rendering only.
			cmds = append(cmds, viewport.Sync(m.viewport))
		}

	case errMsg:
		m.err = msg
		m.data = nil
		m.links = nil
		m.cursor = 0
		return m, nil
	case TeaResponse:
		m.cursor = 0
		m.data = strings.Split(msg.Body, "\n")
		content, links := parseContent(m.data, m.cursor)
		m.links = links
		m.viewport.SetContent(content)
		m.url = msg.URL
		m.textInput.SetValue(msg.URL)
		if msg.AddHist {
			m.history.Add(msg.URL)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if useHighPerformanceRenderer {
		cmds = append(cmds, cmd)
	}
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// // p := termenv.ColorProfile()
	// var s strings.Builder

	// s.WriteString(m.textInput.View())
	// s.WriteString("\n\n")

	// if m.err != nil {
	// 	s.WriteString(m.err.Error())
	// }

	if !m.ready {
		return "\n  Initalizing..."
	}

	headerMid := "│ " + m.history.Status() + " ├"
	headerMid += strings.Repeat("─", m.viewport.Width-runewidth.StringWidth(headerMid))

	footerMid := fmt.Sprintf("┤ %3.f%% │", m.viewport.ScrollPercent()*100)
	gapSize := m.viewport.Width - runewidth.StringWidth(footerMid)
	footerMid = strings.Repeat("─", gapSize) + footerMid

	return fmt.Sprintf("%s\n\n%s\n%s\n%s", m.textInput.View(), headerMid, m.viewport.View(), footerMid)
}

func initialModel(surl string) model {
	i := textinput.NewModel()
	i.Width = 200
	i.SetValue(surl)
	i.Focus()

	return model{
		data:      nil,
		textInput: i,
		startURL:  surl,
		history:   &History{},

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: make(map[int]struct{}),
	}
}

func run() error {
	surl := flag.String("url", "", "URL to start with")
	flag.Parse()

	p := tea.NewProgram(initialModel(*surl))
	p.EnterAltScreen()
	defer p.ExitAltScreen()
	p.EnableMouseCellMotion()
	defer p.DisableMouseCellMotion()
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
