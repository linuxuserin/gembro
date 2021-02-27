package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func main() {
	cacheDir := flag.String("cache-dir", "", "Directory to store cache files (like cert info and bookmarks)")
	url := flag.String("url", "", "URL to start with")
	debug := flag.String("debug-url", "", "Debug an URL")
	logFile := flag.String("log-file", "", "File to output log to")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if *debug != "" {
		if err := debugURL(*cacheDir, *debug); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open log file: %s\n", err)
			os.Exit(1)
		}
		defer f.Close()
		log.SetOutput(f)
	} else {
		log.SetOutput(io.Discard)
	}

	if err := run(*cacheDir, *url); err != nil {
		log.Fatal(err)
	}
}

func debugURL(cacheDir, url string) error {
	u, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("could not parse URL: %w", err)
	}
	if u.Scheme != "gemini" {
		return fmt.Errorf("non-gemini scheme")
	}
	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName))
	if err != nil {
		return err
	}
	fmt.Printf("Start loading %q\n", url)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	resp, err := client.LoadURL(ctx, *u, false)
	if err != nil {
		return err
	}

	fmt.Println(resp.Header)
	return nil
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
		if msg.Alt && len(msg.Runes) == 1 && '1' <= msg.Runes[0] && msg.Runes[0] <= '9' {
			num := int(msg.Runes[0] - '0')
			if num <= len(m.tabs) {
				return m.selectTab(num - 1)
			}
			if len(m.tabs) < 9 {
				return m.openNewTab("", true)
			}
		}
	case OpenNewTabEvent:
		return m.openNewTab(msg.URL, msg.Switch)
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

func (m model) openNewTab(url string, switchTo bool) (model, tea.Cmd) {
	var cmd tea.Cmd
	if len(m.tabs) < 9 {
		m.tabs = append(m.tabs, NewTab(m.client, url, m.bookmarks))
		if switchTo {
			cmd = fireEvent(SelectTabEvent{Tab: len(m.tabs) - 1})
		}
	}
	return m, cmd
}

type LoadURLEvent struct {
	URL        string
	AddHistory bool
}

type GoBackEvent struct{}
type GoForwardEvent struct{}

type ToggleBookmarkEvent struct {
	URL, Title string
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
