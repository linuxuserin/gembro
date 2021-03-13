package gopher

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	neturl "net/url"
	"strings"

	"git.sr.ht/~rafael/gembro/text"
)

type Response struct {
	Data []byte
	Type byte
	URL  string
}

const TextWidth = 80

func ToANSI(data []byte, typ byte) (s string, links text.Links) {
	var buf strings.Builder
	switch typ {
	case '0', 'h':
		return text.Wrap(string(data), TextWidth), links
	}
	var ypos int
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			fmt.Fprintln(&buf)
			ypos++
			continue
		}
		f := strings.Split(line[1:], "\t")
		switch line[0] {
		case 'i':
			fmt.Fprintf(&buf, "%s\n", text.Color(f[0], text.Ch2))
		case '1', '0', 'h':
			var url string
			external := strings.HasPrefix(f[1], "URL:")
			if external {
				url = f[1][4:]
			} else {
				url = fmt.Sprintf("gopher://%s:%s/%c%s", f[2], f[3], line[0], f[1])
			}
			count := links.Count() + 1
			links.Add(ypos, count, url, f[0])
			fmt.Fprintf(&buf, "%d> %s", count, text.Color(f[0], text.Clink))
			switch {
			case external:
				fmt.Fprintf(&buf, " (%s)\n", strings.Split(url, "://")[0])
			case line[0] == '0':
				fmt.Fprintln(&buf, " (text)")
			case line[0] == 'h':
				fmt.Fprintln(&buf, " (html)")
			default:
				fmt.Fprintln(&buf)
			}
		default:
			fmt.Fprintf(&buf, "%s\n", f[0])
		}
		ypos++
	}
	return buf.String(), links
}

func LoadURL(ctx context.Context, url neturl.URL) (*Response, error) {
	log.Printf("gopher load: %s", url.String())
	if url.Port() == "" {
		url.Host += ":70"
	}
	var path string
	var typ byte
	if url.Path == "" || url.Path == "/" {
		path = "/"
		typ = '1'
	} else {
		path = url.Path[2:]
		typ = url.Path[1]
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", url.Host)
	if err != nil {
		return nil, fmt.Errorf("could not dial gopher %q: %w", url.Host, err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "%s\r\n", path)
	data, err := io.ReadAll(io.LimitReader(conn, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("error in gopher response: %w", err)
	}
	return &Response{Data: data, URL: url.String(), Type: typ}, nil
}
