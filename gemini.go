package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode"
)

type Header struct {
	Status uint
	Meta   string
}

func readHeader(in io.Reader) (*Header, error) {
	// Header can not be longer than 1024+5 (1024 for meta, 2 for status, 1 for space and 2 for \r\n)
	rdr := bufio.NewReader(io.LimitReader(in, 1024+5))
	line, err := rdr.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}
	var h Header
	if _, err := fmt.Sscanf(line, "%d %s\r\n", &h.Status, &h.Meta); err != nil {
		return nil, fmt.Errorf("could not scan header: %w", err)
	}
	if h.Status > 99 {
		return nil, fmt.Errorf("status too long")
	}
	return &h, nil
}

func readBody(in io.Reader) (string, error) {
	var s strings.Builder
	rdr := bufio.NewReader(io.LimitReader(in, 1024*1024))
	for {
		line, err := rdr.ReadString('\n')
		if line != "" {
			s.WriteString(strings.TrimRight(line, "\r"))
		}
		if err != nil {
			if err != io.EOF {
				return "", fmt.Errorf("read error: %s", err)
			}
			break
		}
	}
	return s.String(), nil
}

type Response struct {
	Body   string
	Header Header
	URL    string
}

func loadURL(surl url.URL) (*Response, error) {
	// const host = "gemini.circumlunar.space"
	// url := fmt.Sprintf("gemini://%s/", host)
	conn, err := tls.Dial("tcp", surl.Hostname()+":1965", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to server: %s", err)
	}
	defer conn.Close()

	// Send URL
	if _, err := fmt.Fprintf(conn, "%s\r\n", surl.String()); err != nil {
		return nil, fmt.Errorf("could not send url: %s", err)
	}

	header, err := readHeader(conn)
	if err != nil {
		return nil, err
	}

	switch header.Status / 10 {
	case 1: // input
		return &Response{Header: *header}, nil
	case 2: // success
		body, err := readBody(conn)
		if err != nil {
			return nil, err
		}
		return &Response{Body: body, Header: *header, URL: surl.String()}, nil
	case 3: // redirect
		return &Response{Header: *header}, nil
	case 4, 5: // temporary/permanent failure
		return &Response{Header: *header}, nil
	case 6: // client certificate required
		return &Response{Header: *header}, nil
	default:
		return nil, fmt.Errorf("unknown response status: %d", header.Status)
	}
}

type Link struct {
	URL  string
	Name string
}

func (l *Link) FullURL(parent string) string {
	if strings.Contains(l.URL, "://") {
		return l.URL
	}
	u, _ := url.Parse(parent)
	u, _ = u.Parse(l.URL)
	return u.String()
}

func ParseLink(line string) (*Link, error) {
	chars := strings.TrimSpace(line[2:])
	if len(chars) == 0 {
		return nil, fmt.Errorf("incorrect format for link")
	}
	idx := strings.IndexFunc(chars, unicode.IsSpace)
	if idx == -1 {
		return &Link{
			URL:  chars,
			Name: chars,
		}, nil
	}
	return &Link{
		URL:  chars[:idx],
		Name: strings.TrimSpace(chars[idx:]),
	}, nil
}
