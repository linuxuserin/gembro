package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"math/rand"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"git.sr.ht/~rafael/gembro/gemini"
	"git.sr.ht/~rafael/gembro/internal/bookmark"
	"git.sr.ht/~rafael/gembro/internal/history"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

const (
	certsName     = "certs.json"
	bookmarksName = "bookmarks.json"
)

const (
	headerHeight = 2
)

const (
	certName = "cert.crt"
	keyName  = "cert.key"
)

var builtinBookmarks = []bookmark.Bookmark{
	{URL: "gemini://gemini.circumlunar.space/", Name: "Project Gemini"},
	{URL: "gemini://gus.guru/", Name: "Gemini Universal Search"},
	{URL: "gemini://medusae.space/", Name: "A gemini directory"},
}

func main() {
	cacheDir := flag.String("cache-dir", "", "Directory to store cache files (like cert info and bookmarks)")
	debug := flag.String("debug-url", "", "Debug an URL")
	logFile := flag.String("log-file", "", "File to output log to")
	gen := flag.String("generate-certificate", "", "Generate a client certificate with given name")
	flag.Parse()

	var url string
	if len(flag.Args()) > 0 {
		url = flag.Arg(0)
	}

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

	if *cacheDir == "" {
		c, err := os.UserCacheDir()
		if err != nil {
			log.Print(err)
			c = ".cache"
		}
		*cacheDir = filepath.Join(c, "gembro")
		if _, err := os.Stat(*cacheDir); os.IsNotExist(err) {
			if err := os.MkdirAll(*cacheDir, 0777); err != nil {
				log.Print(err)
			}
		}
		log.Printf("cache dir: %s", *cacheDir)
	}

	if *gen != "" {
		certFile := filepath.Join(*cacheDir, certName)
		keyFile := filepath.Join(*cacheDir, keyName)
		if err := generateCertificate(*gen, certFile, keyFile); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("Key file at: %q\nCert file at: %q\nGembro will automatically load this certificate.\n",
			keyFile, certFile)
		return
	}

	if err := run(*cacheDir, url); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func generateCertificate(name, certFile, keyFile string) error {
	if _, err := os.Stat(certFile); !os.IsNotExist(err) {
		return fmt.Errorf("certificate file already exists at %q", certFile)
	}
	if _, err := os.Stat(keyFile); !os.IsNotExist(err) {
		return fmt.Errorf("key file already exists at %q", keyFile)
	}

	rand.Seed(time.Now().UnixNano())
	err := gemini.GenerateClientCertificate(certFile, keyFile, x509.Certificate{
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(5, 0, 0),
		// you have to generate a different serial number each execution
		SerialNumber: big.NewInt(rand.Int63()),
		Subject: pkix.Name{
			CommonName:   name,
			Organization: []string{name},
		},
		BasicConstraintsValid: true,
	})
	if err != nil {
		return fmt.Errorf("could not generate certificate: %w", err)
	}
	return nil
}

func debugURL(cacheDir, url string) error {
	u, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("could not parse URL: %w", err)
	}
	if u.Scheme != "gemini" {
		return fmt.Errorf("non-gemini scheme")
	}
	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName), nil)
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
	certFile := filepath.Join(cacheDir, certName)
	keyFile := filepath.Join(cacheDir, keyName)
	ccert, err := tls.LoadX509KeyPair(certFile, keyFile)
	var cert *tls.Certificate
	if err != nil {
		log.Printf("did not load client certificate: %s", err)
		log.Printf("generate certificate with -generate-certificate if client certificate is needed")
	} else {
		cert = &ccert
	}

	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName), cert)
	if err != nil {
		return err
	}

	bs, err := bookmark.Load(filepath.Join(cacheDir, bookmarksName))
	if err != nil {
		return err
	}

	historyPath := filepath.Join(cacheDir, "history.json")
	tabs, seqID, err := loadTabs(historyPath, client, bs, url)
	if err != nil {
		return err
	}
	p := tea.NewProgram(model{
		client:      client,
		bookmarks:   bs,
		sequenceID:  seqID,
		tabs:        tabs,
		historyPath: historyPath,
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

type TabEvent interface {
	Tab() tabID
}

type model struct {
	tabs          []Tab
	currentTab    int
	historyPath   string
	lastWindowMsg tea.WindowSizeMsg
	client        *gemini.Client
	bookmarks     *bookmark.Store
	sequenceID    tabID
}

type QuitEvent struct{}

func (m model) Init() tea.Cmd {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	return func() tea.Msg {
		<-sigs
		return QuitEvent{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("Event: %T", msg)
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lastWindowMsg = msg
	case QuitEvent:
		if err := m.saveHistory(m.historyPath); err != nil {
			log.Print(err)
		}
		return m, tea.Quit
	case tea.KeyMsg:
		keys := msg.String()
		switch keys {
		case "ctrl+c":
			return m, fireEvent(QuitEvent{})
		case "tab":
			num := (m.currentTab + 1) % len(m.tabs)
			return m.selectTab(num)
		case "shift+tab":
			num := (m.currentTab + len(m.tabs) - 1) % len(m.tabs)
			return m.selectTab(num)
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
	case TabEvent:
		for i, tab := range m.tabs {
			if tab.id != msg.Tab() {
				continue
			}
			log.Printf("Tab event %T for tab %d", msg, tab.id)
			m.tabs[i], cmd = tab.Update(msg)
			break
		}
		return m, cmd
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
		m.sequenceID++
		m.tabs = append(m.tabs, NewTab(m.client, url, 0, m.bookmarks, nil, m.sequenceID))
		if switchTo {
			cmd = fireEvent(SelectTabEvent{Tab: len(m.tabs) - 1})
		}
	}
	return m, cmd
}

type LoadURLEvent struct {
	URL        string
	ScrollPos int
	AddHistory bool
}

type GoBackEvent struct{}
type GoForwardEvent struct{}
type EditSourceEvent struct{}

type ToggleBookmarkEvent struct {
	URL, Title string
}

func (m model) selectTab(tab int) (model, tea.Cmd) {
	if tab < len(m.tabs) {
		m.currentTab = tab
		var cmd tea.Cmd
		m.tabs[m.currentTab], cmd = m.tabs[m.currentTab].Update(m.lastWindowMsg)
		return m, tea.Batch(cmd, spinner.Tick)
	}
	return m, nil
}

func (m model) saveHistory(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not open history file: %w", err)
	}
	defer f.Close()
	for i := range m.tabs {
		tab := &m.tabs[i]
		tab.history.UpdateScroll(tab.viewport.viewport.YOffset)
		if err := tab.history.ToJSON(f); err != nil {
			return fmt.Errorf("could not write history: %w", err)
		}
	}
	return nil
}

func loadTabs(historyPath string, client *gemini.Client, bs *bookmark.Store, startURL string) ([]Tab, tabID, error) {
	seqID := tabID(1)
	f, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Tab{
				NewTab(client, startURL, 0, bs, nil, seqID),
			}, seqID + 1, nil
		}
		return nil, 0, fmt.Errorf("could not load history file: %w", err)
	}
	defer f.Close()

	hs, err := history.FromJSON(f)
	if err != nil {
		log.Printf("Incompatible history file. Ignoring it.")
		return []Tab{
			NewTab(client, startURL, 0, bs, nil, seqID),
		}, seqID + 1, nil
	}
	var tabs []Tab
	for _, h := range hs {
		u := startURL
		scrollPos := 0
		if u == "" {
			u, scrollPos = h.Current()
		}
		tab := NewTab(client, u, scrollPos, bs, h, seqID)
		tabs = append(tabs, tab)
		seqID++
	}
	return tabs, seqID, nil
}
