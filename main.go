package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/gemini"
	"git.sr.ht/~rafael/gemini-browser/internal/history"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

const certsName = "certs.json"
const (
	headerHeight = 1
	footerHeight = 1
)

func main() {
	f, err := os.OpenFile("out.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cacheDir := flag.String("cache-dir", "", "Directory to store cache files")
	url := flag.String("url", "", "URL to start with")
	flag.Parse()

	if err := run(*cacheDir, *url); err != nil {
		log.Fatal(err)
	}
}

func run(cacheDir, url string) error {
	data, _ := os.ReadFile("spacewalk.gmi")

	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName))
	if err != nil {
		return err
	}

	ti := textinput.NewModel()
	ti.Placeholder = ""
	ti.CharLimit = 156
	ti.Width = 50

	s := spinner.NewModel()
	s.Spinner = spinner.Line

	p := tea.NewProgram(model{
		mode:     modePage,
		content:  string(data),
		client:   client,
		title:    "Home",
		history:  &history.History{},
		input:    ti,
		spinner:  s,
		startURL: url,
	})
	p.EnterAltScreen()
	defer p.ExitAltScreen()
	p.EnableMouseCellMotion()
	defer p.DisableMouseCellMotion()

	return p.Start()
}

type mode int

const (
	modePage mode = iota
	modeInput
	modeMessage
	modeNavigate
)

type model struct {
	mode       mode
	client     *gemini.Client
	content    string
	ready      bool
	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model
	title      string
	links      []linkPos
	history    *history.History
	searchURL  string
	inputQuery string
	cancel     context.CancelFunc
	startURL   string
	loading    bool
}

var loadURLError = errors.New("could not load URL")

func (m model) loadURL(url string, addHist bool) func() tea.Msg {
	return func() tea.Msg {
		log.Print(url)
		u, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		if u.Scheme != "gemini" {
			return nil
		}
		if m.cancel != nil {
			m.cancel()
		}
		var ctx context.Context
		ctx, m.cancel = context.WithCancel(context.Background())
		resp, err := m.client.LoadURL(ctx, *u, true)
		if err != nil {
			return loadURLError
		}
		if addHist {
			m.history.Add(u.String())
		}
		return resp
	}
}

func (m model) Init() tea.Cmd {
	return spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case error:
		if errors.Is(msg, loadURLError) {
			m.loading = false
		}
		log.Printf("%[1]T %[1]v", msg)
		m.mode = modeMessage
		m.inputQuery = msg.Error()
		return m, nil
	case tea.KeyMsg:
		log.Print(msg.String())
		skey := msg.String()
		switch m.mode {
		case modeInput:
			switch skey {
			case "enter":
				m.mode = modePage
				m.input.Blur()
				m.loading = true
				return m, tea.Batch(spinner.Tick, m.loadURL(fmt.Sprintf("%s?%s", m.searchURL, neturl.QueryEscape(m.input.Value())), true))
			}
		case modeMessage:
			switch skey {
			case "enter":
				m.mode = modePage
				m.input.Blur()
				return m, nil
			}
		case modeNavigate:
			switch skey {
			case "enter":
				m.mode = modePage
				m.input.Blur()
				m.loading = true
				return m, tea.Batch(spinner.Tick, m.loadURL(m.input.Value(), true))
			}
		case modePage:
			switch skey {
			case "g":
				m.mode = modeNavigate
				m.input.SetValue("")
				m.input.Focus()
				return m, nil
			case "left":
				if url, ok := m.history.Back(); ok {
					m.loading = true
					return m, tea.Batch(spinner.Tick, m.loadURL(url, false))
				}
			case "right":
				if url, ok := m.history.Forward(); ok {
					m.loading = true
					return m, tea.Batch(spinner.Tick, m.loadURL(url, false))
				}
			case "ctrl+c", "q":
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		verticalMargins := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.Model{Width: msg.Width, Height: msg.Height - verticalMargins}
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false
			m.loading = true
			cmds = append(cmds, m.loadURL(m.startURL, true), spinner.Tick)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}
	case tea.MouseMsg:
		log.Printf("Mouse event: %v", msg)
		switch msg.Type {
		case tea.MouseLeft:
			ypos := m.viewport.YOffset + msg.Y - headerHeight
			log.Printf("Ypos: %v %v %v", m.viewport.YOffset, msg.Y, ypos)
			for _, l := range m.links {
				if l.y == ypos {
					m.loading = true
					cmds = append(cmds, m.loadURL(l.url, true), spinner.Tick)
					break
				}
			}
		case tea.MouseRight:
			if url, ok := m.history.Back(); ok {
				m.loading = true
				return m, tea.Batch(m.loadURL(url, false), spinner.Tick)
			}
		}
	case *gemini.Response:
		m.loading = false
		switch msg.Header.Status {
		default:
			log.Print(msg.Header)
		case 1:
			m.mode = modeInput
			m.searchURL = msg.URL
			m.inputQuery = msg.Header.Meta
			m.input.SetValue("")
			m.input.Focus()
		case 4, 5, 6:
			m.mode = modeMessage
			m.inputQuery = fmt.Sprintf("Error: %s", msg.Header.Meta)
			m.input.SetValue("")
			m.input.Focus()
		case 2:
			body, err := msg.GetBody()
			if err != nil {
				log.Print(err)
				return m, nil
			}
			u, _ := neturl.Parse(msg.URL)
			var s string
			s, m.links, m.title = parseContent(body, m.viewport.Width, *u)
			m.viewport.SetContent(s)
			m.viewport.GotoTop()
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, _ = m.viewport.Update(msg)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	switch m.mode {
	case modeInput:
		return fmt.Sprintf("%s %s\n\nPress ENTER to continue", m.inputQuery, m.input.View())
	case modeMessage:
		return fmt.Sprintf("%s\n\nPress ENTER to continue", m.inputQuery)
	case modeNavigate:
		return fmt.Sprintf("Goto %s\n\nPress ENTER to continue", m.input.View())
	default:
		if !m.ready {
			return "\n  Initalizing..."
		}

		header := m.title
		if m.loading {
			header += fmt.Sprintf(" :: %s", m.spinner.View())
		}
		footer := fmt.Sprintf(" %3.f%%", m.viewport.ScrollPercent()*100)
		footerLead := "Back (RMB) Forward (->) "
		gapSize := m.viewport.Width - runewidth.StringWidth(footer) - runewidth.StringWidth(footerLead)
		footer = footerLead + strings.Repeat("─", gapSize) + footer

		return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
	}
}
