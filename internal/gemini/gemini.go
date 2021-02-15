package gemini

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/html/charset"
)

type Header struct {
	Status, StatusDetail uint8
	Meta                 string
}

func readHeader(in *bufio.Reader) (*Header, error) {
	line, err := in.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}
	var h Header
	if len(line) < 2 {
		return nil, fmt.Errorf("header too short")
	}
	if '1' > line[0] || line[0] > '6' {
		return nil, fmt.Errorf("malformed header")
	}
	h.Status = line[0] - '0'
	if '0' > line[1] || line[1] > '9' {
		return nil, fmt.Errorf("malformed header")
	}
	h.StatusDetail = line[1] - '0'
	h.Meta = strings.TrimSpace(line[2:])
	if len(h.Meta) > 1024 {
		return nil, fmt.Errorf("meta too long")
	}
	return &h, nil
}

func readBody(in *bufio.Reader) (string, error) {
	var s strings.Builder
	for {
		line, err := in.ReadString('\n')
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

func (r *Response) GetBody() (string, error) {
	e, _, certain := charset.DetermineEncoding(nil, r.Header.Meta)
	if !certain {
		return r.Body, nil
	}
	return e.NewDecoder().String(r.Body)
}

func LoadURL(surl url.URL) (*Response, error) {
	port := surl.Port()
	if port == "" {
		port = "1965"
	}
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", surl.Hostname(), port), &tls.Config{
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

	rdr := bufio.NewReader(io.LimitReader(conn, 1024*1024))
	header, err := readHeader(rdr)
	if err != nil {
		return nil, err
	}

	resp := &Response{Header: *header, URL: surl.String()}
	switch header.Status {
	case 1: // input
		return resp, nil
	case 2: // success
		body, err := readBody(rdr)
		if err != nil {
			return nil, err
		}
		resp.Body = body
		return resp, nil
	default:
		return resp, nil
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
	u, err := url.Parse(parent)
	if err != nil {
		log.Print(err)
		return ""
	}
	u, err = u.Parse(l.URL)
	if err != nil {
		log.Print(err)
		return ""
	}
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
