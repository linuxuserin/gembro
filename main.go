package main

import (
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
)

func debugURL(surl string) error {
	u, err := url.Parse(surl)
	if err != nil {
		return err
	}
	resp, err := gemini.LoadURL(*u)
	if err != nil {
		return err
	}

	fmt.Printf("%#v", resp.Header)
	return nil
}

type App struct {
	Content        *gtk.Box
	Spin           *gtk.Spinner
	Entry          *gtk.Entry
	bookmarkButton *gtk.Button
	History        history.History
	Bookmarks      *bookmark.BookmarkStore
	currentURL     string
}

func (a *App) setBookmarkIcon(has bool) {
	if has {
		a.bookmarkButton.SetLabel("★")
	} else {
		a.bookmarkButton.SetLabel("☆")
	}
}

func (a *App) prompt(title, surl string) {
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
					a.uiLoadURL(fmt.Sprintf("%s?%s", surl, url.QueryEscape(s)), true)
				}
			}
		})
		d.ShowAll()
	})
}

func (a *App) loadHome() {
	a.Entry.SetText("")
	a.setBookmarkIcon(false)

	var s strings.Builder
	s.WriteString(`<span size="xx-large">Bookmarks</span>` + "\n")
	for _, b := range a.Bookmarks.All() {
		s.WriteString(renderLink(b.URL, b.Name) + "\n")
	}

	if err := a.renderMeta(s.String(), ""); err != nil {
		log.Print(err)
	}
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

func (a *App) renderError(meta string) {
	if err := a.renderMeta(meta, ""); err != nil {
		log.Print(err)
	}
}

func (a *App) renderMeta(meta string, surl string) error {
	l, err := gtk.LabelNew("")
	if err != nil {
		return err
	}
	l.SetSelectable(true)
	l.SetLineWrap(true)
	l.SetHAlign(gtk.ALIGN_START)
	_, _ = l.Connect("activate-link", func(l *gtk.Label, url string) bool {
		if strings.HasPrefix(url, "gemini://") {
			a.uiLoadURL(url, true)
			return true
		}
		return false
	})
	l.SetMarkup(meta)

	_, _ = glib.IdleAdd(func() {
		a.Entry.SetText(surl)
		if surl != "" {
			a.setBookmarkIcon(a.Bookmarks.Contains(a.currentURL))
		} else {
			a.setBookmarkIcon(false)
		}

		w := a.Content
		w.GetChildren().Foreach(func(i interface{}) {
			w.Remove(i.(gtk.IWidget))
		})
		w.Add(l)
		w.ShowAll()
		a.Spin.Stop()
	})
	return nil
}

func (a *App) uiLoadURL(surl string, addHistory bool) {
	a.uiLoadURLDepth(surl, addHistory, 0)
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

func (a *App) uiLoadURLDepth(surl string, addHistory bool, depth int) {
	a.spin(true)
	go func() {
		stopSpinning := true
		defer func() {
			if stopSpinning {
				a.spin(false)
			}
		}()
		u, err := url.Parse(surl)
		if err != nil {
			log.Printf("invalid url: %s", err)
			return
		}
		resp, err := gemini.LoadURL(*u)
		if err != nil {
			log.Print(err)
			return
		}
		switch resp.Header.Status {
		case 1:
			a.prompt(resp.Header.Meta, surl)
			return
		case 2:
			// Continue normally
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
					a.uiLoadURLDepth(u.String(), addHistory, depth+1)
					return
				}

				resp.Body = fmt.Sprintf(
					`# Wrong scheme`+
						"\n\nThe page tried to redirect to a non-gemini URL.\n\nGo there anyway:\n=> %s",
					resp.Header.Meta)
			} else {
				resp.Body = fmt.Sprintf(
					`# 👹 Welcome to the Web From Hell 👹`+
						"\n\nThe page redirected more than 5 times.\n\nRedirect (up to) 5 more times:\n=> %s",
					resp.Header.Meta)
			}
		default:
			a.renderError(fmt.Sprintf("Unknown response with code %d and meta %q",
				resp.Header.Status, resp.Header.Meta))
			return
		}
		if addHistory {
			a.History.Add(resp.URL)
		}
		a.currentURL = resp.URL

		body, err := resp.GetBody()
		if err != nil {
			a.renderError(err.Error())
			return
		}
		lines := strings.Split(body, "\n")
		var mono bool
		for i, line := range lines {
			if strings.HasPrefix(line, "```") {
				mono = !mono
				if mono {
					lines[i] = `<tt>`
				} else {
					lines[i] = `</tt>`
				}
				continue
			}
			if !mono && strings.HasPrefix(line, "=>") {
				l, err := gemini.ParseLink(line)
				if err != nil {
					l = &gemini.Link{URL: "", Name: "Invalid link: " + line}
				}
				rl := renderLink(l.FullURL(resp.URL), l.Name)
				lines[i] = rl
				continue
			}
			line = template.HTMLEscapeString(line)
			if !mono && strings.HasPrefix(line, "# ") {
				lines[i] = fmt.Sprintf(`<span size="xx-large">%s</span>`, line[2:])
				continue
			}
			if !mono && strings.HasPrefix(line, "## ") {
				lines[i] = fmt.Sprintf(`<span size="x-large">%s</span>`, line[3:])
				continue
			}
			if !mono && strings.HasPrefix(line, "### ") {
				lines[i] = fmt.Sprintf(`<span size="large">%s</span>`, line[4:])
				continue
			}
			lines[i] = line
		}

		err = a.renderMeta(strings.Join(lines, "\n"), a.currentURL)
		if err != nil {
			log.Print(err)
		}
	}()
}

func run() error {
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
	app := App{Bookmarks: bs}

	gtk.Init(nil)
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}
	win.SetTitle(appTitle)
	_, _ = win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	b, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return err
	}
	win.Add(b)

	hb, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return err
	}
	hb.SetMarginTop(5)
	hb.SetMarginBottom(5)
	hb.SetMarginStart(10)
	hb.SetMarginEnd(10)
	b.Add(hb)

	cb, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return err
	}
	app.Content = cb
	cb.SetMarginStart(10)
	cb.SetMarginEnd(10)

	f, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	f.SetVExpand(true)
	f.Add(cb)

	//
	// URL bar
	back, err := gtk.ButtonNewWithLabel("⏴")
	if err != nil {
		return err
	}
	_, _ = back.Connect("clicked", func() {
		if surl, ok := app.History.Back(); ok {
			app.uiLoadURL(surl, false)
		}
	})
	hb.Add(back)

	forward, err := gtk.ButtonNewWithLabel("⏵")
	if err != nil {
		return err
	}
	_, _ = forward.Connect("clicked", func() {
		if surl, ok := app.History.Forward(); ok {
			app.uiLoadURL(surl, false)
		}
	})
	hb.Add(forward)

	e, err := gtk.EntryNew()
	if err != nil {
		return err
	}
	app.Entry = e
	_, _ = e.Connect("activate", func() {
		s, _ := e.GetText()
		app.uiLoadURL(s, true)
	})
	e.SetHExpand(true)
	e.SetText(*surl)
	hb.Add(e)

	bookmark, err := gtk.ButtonNewWithLabel("☆")
	if err != nil {
		return err
	}
	app.bookmarkButton = bookmark
	_, _ = bookmark.Connect("clicked", func() {
		if app.currentURL == "" {
			return
		}
		if app.Bookmarks.Contains(app.currentURL) {
			if err := app.Bookmarks.Remove(app.currentURL); err != nil {
				log.Panic(err)
				return
			}
			app.setBookmarkIcon(false)
			return
		}
		if err := app.Bookmarks.Add(app.currentURL, app.currentURL); err != nil {
			log.Panic(err)
			return
		}
		app.setBookmarkIcon(true)
	})
	hb.Add(bookmark)

	home, err := gtk.ButtonNewWithLabel("⌂")
	if err != nil {
		return err
	}
	_, _ = home.Connect("clicked", func() {
		app.loadHome()
	})
	hb.Add(home)

	spin, err := gtk.SpinnerNew()
	if err != nil {
		return err
	}
	app.Spin = spin
	hb.Add(spin)

	// End URL bar
	//

	b.Add(f)

	// Set the default window size.
	win.SetDefaultSize(800, 600)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Recursively show all widgets contained in this window.
	win.ShowAll()

	if *surl != "" {
		go func() {
			_, _ = glib.IdleAdd(func() {
				app.uiLoadURL(*surl, true)
			})
		}()
	} else {
		app.loadHome()
	}

	gtk.Main()

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
