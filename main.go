package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/internal/gemini"
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

func format(html string, args ...string) string {
	nargs := make([]interface{}, len(args))
	for i, arg := range args {
		nargs[i] = template.HTMLEscapeString(arg)
	}
	return fmt.Sprintf(html, nargs...)
}

var style = `
	body {background: black; color: white; font-family: monospace}
	.h1 {color: red; font-weight: bold}
	.h2 {color: yellow; font-weight: bold}
	.h3 {color: fuchsia; font-weight: bold}
	a {color: cornflowerblue; text-decoration: none}
	pre {margin:0}
	span, blockquote {white-space: pre-wrap}
	blockquote { font-style: italic }
	.outer {margin: 0 auto; max-width: 600px; padding-top: 20px; overflow-wrap: anywhere}
`

var homeGemText = `# Gemini Proxy

## Useful links

=> gemini://gemini.circumlunar.space/ Project Gemini
=> gemini://gus.guru/ GUS - Gemini Universal Search
=> gemini://dioskouroi.xyz/top Gemgohane: GEmini GOpher HAckerNEws Mirror`

func geminiToHTML(input, parentURL string) string {
	lines := strings.Split(input, "\n")
	pageTitle := ""
	var mono bool
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "# ") {
			lines[i] = format(`<span class="h1">%s</span><br>`, line)
			if pageTitle == "" {
				pageTitle = line[2:]
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			lines[i] = format(`<span class="h2">%s</span><br>`, line)
			continue
		}
		if strings.HasPrefix(line, "### ") {
			lines[i] = format(`<span class="h3">%s</span><br>`, line)
			continue
		}
		if strings.HasPrefix(line, ">") {
			lines[i] = format(`<blockquote>%s</blockquote><br>`, line[1:])
			continue
		}
		if strings.HasPrefix(line, "```") {
			mono = !mono
			if mono {
				lines[i] = "<pre>"
			} else {
				lines[i] = "</pre>"
			}
			continue
		}
		if strings.HasPrefix(line, "=>") {
			link, err := gemini.ParseLink(line)
			if err != nil {
				link = &gemini.Link{URL: "", Name: "Invalid link"}
			}
			furl := link.FullURL(parentURL)
			if strings.HasPrefix(furl, "gemini://") {
				furl = fmt.Sprintf("?url=%s", furl)
				lines[i] = format(`<a href="%[1]s" title="%[1]s">%[2]s</a><br>`,
					furl, link.Name)
			} else {
				lines[i] = format(`<a href="%[1]s" title="%[1]s" target="_blank">%[2]s</a><br>`, furl, link.Name)
			}
			continue
		}
		lines[i] = format(`<span>%s</span><br>`, line)
	}
	if mono {
		lines = append(lines, "</pre>")
	}
	if pageTitle == "" {
		pageTitle = parentURL
	}
	return fmt.Sprintf(`<!doctype html><html><head><title>%s</title><style>%s</style></head>`+
		`<body><div class="outer">%s</div></body></html>`,
		pageTitle, style, strings.Join(lines, ""))
}

func run() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	durl := flag.String("debug", "", "Debug URL")
	host := flag.String("host", "localhost:8080", "Debug URL")
	flag.Parse()

	if *durl != "" {
		return debugURL(*durl)
	}

	// app := App{Bookmarks: bs, cancelFunc: func() {}}
	return http.ListenAndServe(*host, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rurl := r.FormValue("url")
		if rurl == "" {
			w.Header().Set("Content-type", "text/html; charset=utf-8")
			fmt.Fprint(w, geminiToHTML(homeGemText, ""))
			return
		}

		u, err := url.Parse(rurl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := gemini.LoadURL(r.Context(), *u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch resp.Header.Status {
		// case 1:
		case 2:
			b, err := resp.GetBody()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.FormValue("src") == "1" {
				fmt.Fprint(w, b)
				return
			}
			w.Header().Set("Content-type", "text/html; charset=utf-8")
			fmt.Fprint(w, geminiToHTML(b, rurl))
		case 3:
			u, err := u.Parse(resp.Header.Meta)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("?url=%s", u), http.StatusMovedPermanently)
		default:
			http.Error(w, "Nothing to do", http.StatusBadRequest)
		}
	}))
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
