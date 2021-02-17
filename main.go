package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/internal/gemini"
)

func debugURL(url string) error {
	u, err := neturl.Parse(url)
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
	body {background:black;color:white;font-family:'Source Code Pro', monospace;line-height:150%;font-size:1em;}
	.h1 {color: red; font-weight: bold}
	.h2 {color: yellow; font-weight: bold}
	.h3 {color: fuchsia; font-weight: bold}
	a {color: cornflowerblue; text-decoration: none}
	pre {margin:0}
	blockquote { font-style: italic; margin: 0; }
	.outer {margin: 0 auto; max-width: 600px; padding-top: 20px; overflow-wrap: anywhere;white-space:pre-wrap}
	pre{color:palegoldenrod}
`

var homeGemText = `# Gemini Proxy

## Useful links

=> gemini://gemini.circumlunar.space/ Project Gemini
=> gemini://gus.guru/ GUS - Gemini Universal Search
=> gemini://dioskouroi.xyz/top Gemgohane: GEmini GOpher HAckerNEws Mirror`

var outerHTML = `<!doctype html><html><head><title>%s</title><style>%s</style></head>` +
	`<body><div class="outer">%s</div></body></html>`

var inputFormHTML = `<form action="" method="GET">
	<input type="hidden" name="url" value="%s">
	<input id="q" name="q" placeholder="%s" autofocus>
	<button>Enter</button>
</form>
`

func inputForm(prompt, url string) string {
	html := fmt.Sprintf(outerHTML, "Requesting input", style, format(inputFormHTML, url, prompt))
	return html
}

func geminiToHTML(input, url string) string {
	lines := strings.Split(input, "\n")
	pageTitle := ""
	var mono bool
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if !mono && strings.HasPrefix(line, "# ") {
			lines[i] = format(`<span class="h1">%s</span>`, line)
			if pageTitle == "" {
				pageTitle = line[2:]
			}
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			lines[i] = format(`<span class="h2">%s</span>`, line)
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
			lines[i] = format(`<span class="h3">%s</span>`, line)
			continue
		}
		if !mono && strings.HasPrefix(line, ">") {
			lines[i] = format(`<blockquote>%s</blockquote>`, line[1:])
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
			furl := link.FullURL(url)
			if strings.HasPrefix(furl, "gemini://") {
				furl = fmt.Sprintf("?url=%s", furl)
				lines[i] = format(`<a href="%[1]s" title="%[1]s">%[2]s</a>`,
					furl, link.Name)
			} else {
				lines[i] = format(`<a href="%[1]s" title="%[1]s" target="_blank">%[2]s</a>`, furl, link.Name)
			}
			continue
		}
		if mono {
			lines[i] = format(`%s`, line)
			continue
		}
		lines[i] = format(`<span>%s</span>`, line)
	}
	if mono {
		lines = append(lines, "</pre>")
	}
	if pageTitle == "" {
		pageTitle = url
	}
	return fmt.Sprintf(outerHTML,
		pageTitle, style, strings.Join(lines, "\n"))
}

func errorResponse(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	fmt.Fprintf(w, outerHTML, "Error", style, format("<p>%s</p>", msg))
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
		if q := r.FormValue("q"); q != "" {
			rurl = fmt.Sprintf("%s?%s", rurl, neturl.QueryEscape(q))
			http.Redirect(w, r, fmt.Sprintf("?url=%s", neturl.QueryEscape(rurl)), http.StatusMovedPermanently)
			return
		}
		u, err := neturl.Parse(rurl)
		if err != nil {
			errorResponse(w, "Invalid URL", http.StatusBadRequest)
			return
		}
		resp, err := gemini.LoadURL(r.Context(), *u)
		if err != nil {
			log.Print(err)
			errorResponse(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		switch resp.Header.Status {
		case 1:
			w.Header().Set("Content-type", "text/html; charset=utf-8")
			fmt.Fprint(w, inputForm(resp.Header.Meta, rurl))
		case 2:
			b, err := resp.GetBody()
			if err != nil {
				errorResponse(w, err.Error(), http.StatusBadRequest)
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
				errorResponse(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("?url=%s", u), http.StatusMovedPermanently)
		case 4:
			errorResponse(w, fmt.Sprintf("Temporary failure: %s", resp.Header.Meta), http.StatusServiceUnavailable)
		case 5:
			if resp.Header.StatusDetail == 1 {
				errorResponse(w, "Page not found", http.StatusNotFound)
				return
			}
			errorResponse(w, fmt.Sprintf("Permanent failure: %s", resp.Header.Meta), http.StatusServiceUnavailable)
		case 6:
			errorResponse(w, fmt.Sprintf("Client certificate required: %s", resp.Header.Meta), http.StatusBadRequest)
		default:
			log.Printf("could not parse response for %q: %#v", u.String(), resp)
			errorResponse(w, "Unknown error", http.StatusTeapot)
		}
	}))
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
