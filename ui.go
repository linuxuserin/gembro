package main

import (
	"fmt"
	"log"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

func (app *App) loadURLBar(hb *gtk.Box, startURL string) error {
	back, err := gtk.ButtonNewWithLabel("⏴")
	if err != nil {
		return err
	}
	_, _ = back.Connect("clicked", app.clickBack)
	hb.Add(back)

	forward, err := gtk.ButtonNewWithLabel("⏵")
	if err != nil {
		return err
	}
	_, _ = forward.Connect("clicked", app.clickForward)
	hb.Add(forward)

	e, err := gtk.EntryNew()
	if err != nil {
		return err
	}
	app.Entry = e
	_, _ = e.Connect("activate", app.activateURLbar)
	e.SetHExpand(true)
	e.SetText(startURL)
	hb.Add(e)

	bookmark, err := gtk.ButtonNewWithLabel("☆")
	if err != nil {
		return err
	}
	app.bookmarkButton = bookmark
	_, _ = bookmark.Connect("clicked", app.clickBookmark)
	hb.Add(bookmark)

	home, err := gtk.ButtonNewWithLabel("⌂")
	if err != nil {
		return err
	}
	_, _ = home.Connect("clicked", func() {
		app.gotoURL(homeURL, true)
	})
	hb.Add(home)

	spin, err := gtk.SpinnerNew()
	if err != nil {
		return err
	}
	app.Spin = spin
	hb.Add(spin)
	return nil
}

func (app *App) loadMainUI(startURL string) error {
	gtk.Init(nil)
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}
	win.SetTitle(appTitle)
	_, _ = win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	outerBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return err
	}
	win.Add(outerBox)

	urlBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return err
	}
	urlBox.SetMarginTop(5)
	urlBox.SetMarginBottom(5)
	urlBox.SetMarginStart(10)
	urlBox.SetMarginEnd(10)
	outerBox.Add(urlBox)

	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return err
	}
	contentBox.SetMarginStart(10)
	contentBox.SetMarginEnd(10)
	contentBuf, textView, err := createTextView(contentBox)
	if err != nil {
		return err
	}
	_, _ = textView.Connect("button-release-event", app.clickTextBox)
	app.content = contentBuf
	app.textView = textView
	app.tags = map[string]*gtk.TextTag{
		"mono": app.content.CreateTag("mono", map[string]interface{}{
			"family": "Monospace",
		}),
		"link": app.content.CreateTag("blue", map[string]interface{}{
			"foreground": "#58a6ff",
			"underline":  pango.UNDERLINE_SINGLE,
		}),
		"h1": app.content.CreateTag("h1", map[string]interface{}{
			"scale": float64(pango.SCALE_XX_LARGE),
		}),
		"h2": app.content.CreateTag("h2", map[string]interface{}{
			"scale": float64(pango.SCALE_X_LARGE),
		}),
		"h3": app.content.CreateTag("h3", map[string]interface{}{
			"scale": float64(pango.SCALE_LARGE),
		}),
	}

	// l, err := gtk.LabelNew("")
	// if err != nil {
	// 	return err
	// }
	// l.SetSelectable(true)
	// l.SetLineWrap(true)
	// l.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	// l.SetHAlign(gtk.ALIGN_START)
	// _, _ = l.Connect("activate-link", func(l *gtk.Label, url string) bool {
	// 	if strings.HasPrefix(url, "gemini://") {
	// 		app.gotoURL(url, true)
	// 		return true
	// 	}
	// 	return false
	// })
	// app.label = l
	// contentBox.Add(l)

	f, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	f.SetVExpand(true)
	f.Add(contentBox)

	if err := app.loadURLBar(urlBox, startURL); err != nil {
		return err
	}
	outerBox.Add(f)

	// Set the default window size.
	win.SetDefaultSize(800, 600)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Recursively show all widgets contained in this window.
	win.ShowAll()

	if startURL != "" {
		go func() {
			_, _ = glib.IdleAdd(func() {
				app.gotoURL(startURL, true)
			})
		}()
	} else {
		app.gotoURL(homeURL, true)
	}

	gtk.Main()
	return nil
}

func (app *App) clickTextBox() {
	icurPos, err := app.content.GetProperty("cursor-position")
	if err != nil {
		log.Print(err)
		return
	}
	curPos := icurPos.(int)
	for _, o := range app.links {
		if o.Start <= curPos && curPos <= o.End {
			app.gotoURL(o.URL, true)
			return
		}
	}
}

func (app *App) clickBack() {
	if surl, ok := app.History.Back(); ok {
		app.gotoURL(surl, false)
	}
}

func (app *App) clickForward() {
	if surl, ok := app.History.Forward(); ok {
		app.gotoURL(surl, false)
	}
}

func (app *App) clickBookmark() {
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
}

func (app *App) activateURLbar(e *gtk.Entry) {
	s, _ := e.GetText()
	app.gotoURL(s, true)
}

type Container interface {
	Add(gtk.IWidget)
}

func createLabel(container Container, text string) {
	l, err := gtk.LabelNew(text)
	if err != nil {
		log.Printf("could not create label: %s", err)
		return
	}
	l.SetSelectable(true)
	l.SetLineWrap(true)
	l.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	l.SetHAlign(gtk.ALIGN_START)
	container.Add(l)
}

func createLinkButton(container Container, label, url string, cb func(url string) bool) {
	l, err := gtk.LinkButtonNew(label)
	if err != nil {
		log.Printf("could not create link button: %s", err)
		return
	}
	// l.SetLineWrap(true)
	// l.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	l.SetMarginStart(0)
	l.SetHAlign(gtk.ALIGN_START)
	l.SetUri(url)
	_, _ = l.Connect("activate-link", func() bool {
		return cb(url)
	})
	container.Add(l)
}

func createTextView(container Container) (*gtk.TextBuffer, *gtk.TextView, error) {
	tf, err := gtk.TextBufferNew(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create textbuffer: %s", err)
	}
	t, err := gtk.TextViewNewWithBuffer(tf)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create textview: %s", err)
	}
	t.SetWrapMode(gtk.WRAP_WORD_CHAR)
	t.SetEditable(false)
	t.SetLeftMargin(10)
	t.SetTopMargin(10)
	t.SetBottomMargin(10)
	t.SetRightMargin(10)
	t.SetCursorVisible(false)
	container.Add(t)
	return tf, t, nil
}
