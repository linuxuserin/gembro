package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/gemini"
	"git.sr.ht/~rafael/gemini-browser/internal/bookmark"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

const (
	certsName     = "certs.json"
	bookmarksName = "bookmarks.json"
)

const (
	headerHeight = 2
	footerHeight = 1
)

var builtinBookmarks = []bookmark.Bookmark{
	{URL: "gemini://gemini.circumlunar.space/", Name: "Project Gemini"},
	{URL: "gemini://gus.guru/", Name: "Gemini Universal Search"},
	{URL: "gemini://medusae.space/", Name: "A gemini directory"},
}

type SelectTabEvent struct {
	Tab int
}

type CloseCurrentTabEvent struct{}
type CloseTabEvent struct {
	Tab int
}

type OpenNewTabEvent struct {
	URL string
}

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
	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName))
	if err != nil {
		return err
	}

	bs, err := bookmark.Load(filepath.Join(cacheDir, bookmarksName))
	if err != nil {
		return err
	}

	p := tea.NewProgram(model{
		client:    client,
		bookmarks: bs,
		tabs: []Tab{
			NewTab(client, url, bs),
		},
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
)

type model struct {
	tabs          []Tab
	currentTab    int
	lastWindowMsg tea.WindowSizeMsg
	client        *gemini.Client
	bookmarks     *bookmark.Store
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lastWindowMsg = msg
	case tea.KeyMsg:
		keys := msg.String()
		switch keys {
		case "ctrl+c":
			return m, tea.Quit
		}
		var num int
		n, _ := fmt.Sscanf(keys, "alt+%d", &num)
		if n == 1 && 1 <= num && num <= 9 {
			if num <= len(m.tabs) {
				return m.selectTab(num - 1)
			}
			if len(m.tabs) < 9 {
				m.tabs = append(m.tabs, NewTab(m.client, "", m.bookmarks))
				return m.selectTab(len(m.tabs) - 1)
			}
		}
	case OpenNewTabEvent:
		if len(m.tabs) < 9 {
			m.tabs = append(m.tabs, NewTab(m.client, msg.URL, m.bookmarks))
			return m, cmd
		}
	case CloseCurrentTabEvent:
		return m, fireEvent(CloseTabEvent{Tab: m.currentTab})
	case CloseTabEvent:
		if msg.Tab < len(m.tabs) && len(m.tabs) > 1 {
			m.tabs = append(m.tabs[0:msg.Tab], m.tabs[msg.Tab+1:]...)
			for m.currentTab >= len(m.tabs) {
				m.currentTab--
			}
			return m.selectTab(m.currentTab)
		}
	case SelectTabEvent:
		return m.selectTab(msg.Tab)
	}
	m.tabs[m.currentTab], cmd = m.tabs[m.currentTab].Update(msg)
	return m, cmd
}

func (m model) View() string {
	var buf strings.Builder
	for i := range m.tabs {
		lbl := fmt.Sprintf("[%d]", i+1)
		if m.currentTab == i {
			lbl = termenv.String(lbl).Reverse().String()
		}
		fmt.Fprintf(&buf, "%s ", lbl)
	}
	fmt.Fprintf(&buf, "\n%s", m.tabs[m.currentTab].View())
	return buf.String()
}

func (m model) selectTab(tab int) (model, tea.Cmd) {
	if tab < len(m.tabs) {
		m.currentTab = tab
		var cmd tea.Cmd
		m.tabs[m.currentTab], cmd = m.tabs[m.currentTab].Update(m.lastWindowMsg)
		return m, cmd
	}
	return m, nil
}

func fireEvent(msg tea.Msg) func() tea.Msg {
	return func() tea.Msg {
		return msg
	}
}
