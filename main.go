package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	neturl "net/url"
	"path/filepath"
	"strings"

	"git.sr.ht/~rafael/gemini-browser/gemini"
)

const certsName = "certs.json"

func debugURL(url, cacheDir string, skipVerify bool) error {
	u, err := neturl.Parse(url)
	if err != nil {
		return err
	}
	client, err := gemini.NewClient(filepath.Join(cacheDir, certsName))
	if err != nil {
		return err
	}
	resp, err := client.LoadURL(context.Background(), *u, skipVerify)
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
body {background:black;color:white;font-family:'Source Code Pro',
	monospace;font-size:15px;margin:0}
p {padding:0;margin:0}
.h1 {color: red; font-weight: bold}
.h2 {color: yellow; font-weight: bold}
.h3 {color: fuchsia; font-weight: bold}
a {color: cornflowerblue; text-decoration: none}
blockquote {font-style:italic;margin:0;padding:0 0 0 10px;display:inline-block;border-left:1px solid grey}
.outer {margin: 0 auto;width:800px;padding-top: 20px;white-space:pre}
pre{margin:0;color:palegoldenrod;font-family:'Source Code Pro',monospace}
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

const columns = 80

func geminiToHTML(input, url string) string {
	pageTitle := ""
	var mono bool
	var buf, tempBuf bytes.Buffer
	addTemp := func() {
		s := tempBuf.String()
		if s == "" {
			return
		}
		if mono {
			fmt.Fprintf(&buf, "<pre>\n%s</pre>", s)
		} else {
			fmt.Fprintf(&buf, "<p>%s</p>", s)
		}
		tempBuf.Reset()
	}
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "```") {
			addTemp()
			mono = !mono
			continue
		}
		if mono {
			fmt.Fprintln(&tempBuf, format(`%s`, line))
			continue
		}
		if strings.HasPrefix(line, "=>") {
			addTemp()
			link, err := gemini.ParseLink(line)
			if err != nil {
				link = &gemini.Link{URL: "", Name: "Invalid link"}
			}
			furl := link.FullURL(url)
			name := LineWrap(link.Name, columns)
			if strings.HasPrefix(furl, "gemini://") {
				furl = fmt.Sprintf("?url=%s", furl)
				fmt.Fprintln(&buf, format(`<a href="%[1]s" title="%[1]s">%[2]s</a>`,
					furl, name))
			} else {
				fmt.Fprintln(&buf, format(`<a href="%[1]s" title="%[1]s" target="_blank">%[2]s</a>`,
					furl, name))
			}
			continue
		}

		line = LineWrap(strings.TrimRight(line, "\r"), columns)
		if !mono && strings.HasPrefix(line, "# ") {
			addTemp()
			fmt.Fprintln(&buf, format(`<span class="h1">%s</span>`, line))
			if pageTitle == "" {
				pageTitle = line[2:]
			}
			continue
		}
		if !mono && strings.HasPrefix(line, "## ") {
			addTemp()
			fmt.Fprintln(&buf, format(`<span class="h2">%s</span>`, line))
			continue
		}
		if !mono && strings.HasPrefix(line, "### ") {
			addTemp()
			fmt.Fprintln(&buf, format(`<span class="h3">%s</span>`, line))
			continue
		}
		if !mono && strings.HasPrefix(line, ">") {
			addTemp()
			fmt.Fprintln(&buf, format(`<blockquote>%s</blockquote>`, strings.TrimLeft(line[1:], " ")))
			continue
		}
		fmt.Fprintln(&tempBuf, format(`%s`, line))
	}
	addTemp()
	if pageTitle == "" {
		pageTitle = url
	}
	return fmt.Sprintf(outerHTML,
		pageTitle, style, buf.String())
}

func errorResponse(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	fmt.Fprintf(w, outerHTML, "Error", style, format("<p>%s</p>", msg))
}

func run() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	durl := flag.String("debug", "", "Debug URL")
	dskip := flag.Bool("debug-skip-verify", false, "Skip cert verification for debug URL")
	host := flag.String("host", "localhost:8080", "Debug URL")
	cacheDir := flag.String("cache-dir", "", "Where to store certificate information and other things")
	flag.Parse()

	if *durl != "" {
		return debugURL(*durl, *cacheDir, *dskip)
	}

	client, err := gemini.NewClient(filepath.Join(*cacheDir, certsName))
	if err != nil {
		return err
	}

	// app := App{Bookmarks: bs, cancelFunc: func() {}}
	log.Printf("Server started on %s\n", *host)
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
		resp, err := client.LoadURL(r.Context(), *u, r.FormValue("force") == "1")
		if err != nil {
			if errors.Is(err, gemini.CertChanged) {
				errorResponse(w, "Certificate has changed. Load with &force=1 to continue with new certificate.",
					http.StatusInternalServerError)
				return
			}
			log.Print(err)
			errorResponse(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		switch resp.Header.Status {
		case 1:
			w.Header().Set("Content-type", "text/html; charset=utf-8")
			fmt.Fprint(w, inputForm(resp.Header.Meta, rurl))
		case 2:
			if strings.HasPrefix(resp.Header.Meta, "text/gemini") {
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
			} else {
				w.Header().Set("Content-type", resp.Header.Meta)
				fmt.Fprint(w, resp.Body)
			}
		case 3:
			u, err := u.Parse(resp.Header.Meta)
			if err != nil {
				errorResponse(w, err.Error(), http.StatusBadRequest)
				return
			}
			if u.Scheme == "gemini" {
				http.Redirect(w, r, fmt.Sprintf("?url=%s", u), http.StatusMovedPermanently)
			} else {
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			}
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
