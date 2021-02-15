package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/url"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/internal/bookmark"
	"git.sr.ht/~rafael/gemini-browser/internal/gemini"
	"git.sr.ht/~rafael/gemini-browser/internal/history"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const (
	appTitle = "Gemini Browser"
	homeURL  = "home://"
)

func debugURL(surl string) error {
	u, err := url.Parse(surl)
	if err != nil {
		return err
	}
	resp, err := gemini.LoadURL(context.Background(), *u)
	if err != nil {
		return err
	}

	fmt.Printf("%#v", resp.Header)
	return nil
}

type LinkOffset struct {
	Start, End int
	URL        string
}
type App struct {
	Spin           *gtk.Spinner
	content        *gtk.TextBuffer
	textView       *gtk.TextView
	Entry          *gtk.Entry
	bookmarkButton *gtk.Button
	History        history.History
	Bookmarks      *bookmark.BookmarkStore
	currentURL     string
	tags           map[string]*gtk.TextTag
	links          []LinkOffset
	cancelFunc     context.CancelFunc
}

func (a *App) setBookmarkIcon(has bool) {
	if has {
		a.bookmarkButton.SetLabel("★")
	} else {
		a.bookmarkButton.SetLabel("☆")
	}
}

func (a *App) promptQuery(title, surl string) {
	_, _ = glib.IdleAdd(func() {
		d, err := gtk.DialogNewWithButtons(title, nil, 0)
		if err != nil {
			log.Print(err)
			return
		}
		inp, err := gtk.EntryNew()
		if err != nil {
			log.Print(err)
			return
		}
		d.AddActionWidget(inp, gtk.RESPONSE_ACCEPT)
		if _, err := d.AddButton("OK", gtk.RESPONSE_ACCEPT); err != nil {
			log.Print(err)
			return
		}

		_, _ = d.Connect("response", func(_ *gtk.Dialog, r gtk.ResponseType) {
			defer d.Destroy()
			switch r {
			case gtk.RESPONSE_ACCEPT:
				s, err := inp.GetText()
				if err != nil {
					log.Print(err)
					return
				}
				s = strings.TrimSpace(s)
				if s != "" {
					a.gotoURL(fmt.Sprintf("%s?%s", surl, url.QueryEscape(s)), true)
				}
			}
		})
		d.ShowAll()
	})
}

func (a *App) homeMeta() string {
	var s strings.Builder
	s.WriteString("# Bookmarks\n\n")
	for _, b := range a.Bookmarks.All() {
		s.WriteString(fmt.Sprintf("=> %s %s\n", b.URL, b.Name))
	}
	return s.String()
}

var linkTmpl = template.Must(template.New("foo").Parse(`<a href="{{.URL}}" title="{{.URL}}">{{.Name}}</a> ` +
	`{{if .Type}}({{.Type}}){{end}}`))

func renderLink(surl, name string) string {
	surl = html.EscapeString(surl)
	var typ string
	if !strings.HasPrefix(surl, "gemini://") {
		typ = strings.Split(surl, "://")[0]
	}
	var b strings.Builder
	if err := linkTmpl.Execute(&b, struct {
		URL        template.URL
		Name, Type string
	}{template.URL(surl), name, typ}); err != nil {
		log.Print(err)
		return "Render failure"
	}
	return b.String()
}

func (a *App) renderMeta(meta string, surl string) {
	_, _ = glib.IdleAdd(func() {
		a.Entry.SetText(surl)
		if surl != "" {
			a.setBookmarkIcon(a.Bookmarks.Contains(a.currentURL))
		} else {
			a.setBookmarkIcon(false)
		}

		a.content.SetText("")

		a.links = nil
		var p []string
		var mono bool
		iter := a.content.GetStartIter()
		addP := func() {
			if len(p) == 0 {
				return
			}
			joined := strings.Join(p, "\n")
			if mono {
				a.content.InsertWithTag(iter, joined+"\n", a.tags["mono"])
			} else {
				a.content.Insert(iter, joined+"\n")
			}
			p = nil
		}

		for _, line := range strings.Split(meta, "\n") {
			if strings.HasPrefix(line, "# ") {
				addP()
				a.content.InsertWithTag(iter, line[2:]+"\n", a.tags["h1"])
				continue
			}
			if strings.HasPrefix(line, "## ") {
				addP()
				a.content.InsertWithTag(iter, line[3:]+"\n", a.tags["h2"])
				continue
			}
			if strings.HasPrefix(line, "### ") {
				addP()
				a.content.InsertWithTag(iter, line[4:]+"\n", a.tags["h3"])
				continue
			}
			if strings.HasPrefix(line, "=>") {
				addP()
				link, err := gemini.ParseLink(line)
				if err != nil {
					link = &gemini.Link{URL: "", Name: "Invalid link"}
				}
				furl := link.FullURL(surl)
				_ = furl

				startOffset := iter.GetOffset()
				a.content.InsertWithTag(iter, link.Name, a.tags["link"])
				if !strings.HasPrefix(furl, "gemini://") {
					a.content.Insert(iter, fmt.Sprintf(" (%s)", strings.Split(furl, "://")[0]))
				}
				a.links = append(a.links, LinkOffset{startOffset, iter.GetOffset(), furl})
				a.content.Insert(iter, "\n")
				continue
			}
			if strings.HasPrefix(line, "```") {
				addP()
				mono = !mono
				continue
			}
			p = append(p, line)
		}
		addP()

		a.textView.ShowAll()
		a.Spin.Stop()
	})
}

func (a *App) spin(start bool) {
	_, _ = glib.IdleAdd(func() {
		if start {
			a.Spin.Start()
			return
		}
		a.Spin.Stop()
	})
}

func (a *App) loadURL(surl string) (*gemini.Response, error) {
	switch surl {
	case homeURL:
		var r gemini.Response
		r.Body = a.homeMeta()
		r.Header.Status = 2
		r.URL = surl
		return &r, nil
	default:
		u, err := url.Parse(surl)
		if err != nil {
			return nil, fmt.Errorf("invalid url: %s", err)
		}
		a.cancelFunc()
		ctx, done := context.WithCancel(context.Background())
		a.cancelFunc = done
		return gemini.LoadURL(ctx, *u)
	}
}

func (a *App) gotoURL(surl string, addHistory bool) {
	a.gotoURLDepth(surl, addHistory, 0)
}

func (a *App) gotoURLDepth(surl string, addHistory bool, depth int) {
	a.spin(true)
	go func() {
		stopSpinning := true
		defer func() {
			if stopSpinning {
				a.spin(false)
			}
		}()
		var body string
		resp, err := a.loadURL(surl)
		if err != nil {
			log.Print(err)
			if errors.Is(err, context.Canceled) {
				body = "# Page canceled\n\nYou canceled this page"
			} else {
				body = "# Could not render the page\n\nThe server did not respond with something worthy"
			}
			resp = &gemini.Response{Body: body, Header: gemini.Header{Status: 2}, URL: surl}
		}
		switch resp.Header.Status {
		case 1:
			a.promptQuery(resp.Header.Meta, surl)
			return
		case 2:
			body, err = resp.GetBody()
			if err != nil {
				log.Print(err)
				body = "# Error rendering content\n\nSorry :-("
			}
		case 3: // Redirect
			if depth < 5 {
				stopSpinning = false

				// Get URL to redirect to
				// It might be relative so use current url
				u, err := url.Parse(surl)
				if err != nil {
					return
				}
				u, err = u.Parse(resp.Header.Meta)
				if err != nil {
					return
				}

				if u.Scheme == "gemini" {
					a.gotoURLDepth(u.String(), addHistory, depth+1)
					return
				}

				body = fmt.Sprintf(
					`# Wrong scheme`+
						"\n\nThe page tried to redirect to a non-gemini URL.\n\nGo there anyway:\n=> %s",
					resp.Header.Meta)
			} else {
				body = fmt.Sprintf(
					`# 👹 Welcome to the Web From Hell 👹`+
						"\n\nThe page redirected more than 5 times.\n\nRedirect (up to) 5 more times:\n=> %s",
					resp.Header.Meta)
			}
		case 4:
			body = fmt.Sprintf("# Temporary failure \n\nMessage: %s", resp.Header.Meta)
		case 5:
			if resp.Header.StatusDetail == 1 {
				body = "# Page not found\n\nThis page does not exist 🤷"
			} else {
				body = fmt.Sprintf("# Permanent failure \n\nMessage: %s", resp.Header.Meta)
			}
		case 6:
			body = fmt.Sprintf("# Client certificate required \n\nBecause: %s", resp.Header.Meta)
		default:
			body = "# Unexpected response\n\nNot much that I can do with this, human"
		}
		if addHistory {
			a.History.Add(resp.URL)
		}
		a.currentURL = resp.URL

		a.renderMeta(body, a.currentURL)
	}()
}

func run() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	surl := flag.String("url", "", "URL to start with")
	durl := flag.String("debug", "", "Debug URL")
	flag.Parse()

	if *durl != "" {
		return debugURL(*durl)
	}

	bs, err := bookmark.Load("bookmarks.json")
	if err != nil {
		return err
	}

	app := App{Bookmarks: bs, cancelFunc: func() {}}
	return app.loadMainUI(*surl)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
