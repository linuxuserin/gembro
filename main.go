package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/url"
	"strings"

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
	resp, err := loadURL(*u)
	if err != nil {
		return err
	}

	fmt.Printf("%#v", resp.Header)
	return nil
}

type App struct {
	Content *gtk.Box
	Spin    *gtk.Spinner
	Entry   *gtk.Entry
	History History
}

func (a *App) uiLoadURL(surl string, addHistory bool) {
	a.Spin.Start()
	go func() {
		u, err := url.Parse(surl)
		if err != nil {
			log.Printf("invalid url: %s", err)
			return
		}
		resp, err := loadURL(*u)
		if err != nil {
			log.Print(err)
			return
		}
		if addHistory {
			a.History.Add(resp.URL)
		}

		tmpl := template.Must(template.New("foo").Parse(`<a href="{{.URL}}" title="{{.URL}}">{{.Name}}</a> ` +
			`{{if .Type}}({{.Type}}){{end}}`))
		renderLink := func(l *Link) string {
			var b bytes.Buffer
			surl := html.EscapeString(l.FullURL(resp.URL))
			name := l.Name
			var typ string
			if !strings.HasPrefix(surl, "gemini://") {
				typ = strings.Split(surl, "://")[0]
			}
			if err := tmpl.Execute(&b, struct {
				URL        template.URL
				Name, Type string
			}{template.URL(surl), name, typ}); err != nil {
				log.Print(err)
				return "Render failure"
			}
			return b.String()
		}

		lines := strings.Split(resp.Body, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "=>") {
				l, err := ParseLink(line)
				if err != nil {
					l = &Link{"", "Invalid link: " + line}
				}
				rl := renderLink(l)
				lines[i] = rl
				continue
			}
			line = template.HTMLEscapeString(line)
			if strings.HasPrefix(line, "# ") {
				lines[i] = fmt.Sprintf(`<span size="xx-large">%s</span>`, line[2:])
				continue
			}
			if strings.HasPrefix(line, "## ") {
				lines[i] = fmt.Sprintf(`<span size="x-large">%s</span>`, line[3:])
				continue
			}
			if strings.HasPrefix(line, "### ") {
				lines[i] = fmt.Sprintf(`<span size="large">%s</span>`, line[4:])
				continue
			}
			lines[i] = line
		}

		l, err := gtk.LabelNew("")
		if err != nil {
			log.Print(err)
			return
		}
		l.SetMarkup(strings.Join(lines, "\n"))
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

		_, _ = glib.IdleAdd(func() {
			a.Entry.SetText(resp.URL)

			w := a.Content
			w.GetChildren().Foreach(func(i interface{}) {
				w.Remove(i.(gtk.IWidget))
			})
			w.Add(l)
			w.ShowAll()
			a.Spin.Stop()
		})
	}()
}

func run() error {
	surl := flag.String("url", "", "URL to start with")
	durl := flag.String("debug", "", "Debug URL")
	flag.Parse()

	if *durl != "" {
		return debugURL(*durl)
	}

	app := App{}

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
	back, err := gtk.ButtonNewWithLabel("<-")
	if err != nil {
		return err
	}
	_, _ = back.Connect("clicked", func() {
		if surl, ok := app.History.Back(); ok {
			app.uiLoadURL(surl, false)
		}
	})
	hb.Add(back)

	forward, err := gtk.ButtonNewWithLabel("->")
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

	l, err := gtk.ButtonNew()
	if err != nil {
		return err
	}
	l.SetLabel("Go")
	_, _ = l.Connect("clicked", func() {
		s, _ := e.GetText()
		app.uiLoadURL(s, true)
	})
	hb.Add(l)

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
	}

	gtk.Main()

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
