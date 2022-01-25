package history

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type URL struct {
	url       string
	scrollPos int
}

type History struct {
	sync.Mutex
	urls []URL
	pos  int
}

func (h *History) Add(surl string) {
	h.Lock()
	defer h.Unlock()
	if len(h.urls) == 0 && h.pos == 0 {
		h.pos = -1
	}
	h.urls = append(h.urls[:h.pos+1], URL{surl, 0})
	h.pos = len(h.urls) - 1
}

func (h *History) Back() (string, int, bool) {
	h.Lock()
	defer h.Unlock()
	if h.pos > 0 {
		h.pos--
		u := h.urls[h.pos]
		return u.url, u.scrollPos, true
	}
	return "", 0, false
}

func (h *History) UpdateScroll(pos int) {
	if len(h.urls) == 0 {
		return
	}
	h.urls[h.pos].scrollPos = pos
}

func (h *History) Current() (string, int) {
	h.Lock()
	defer h.Unlock()
	if len(h.urls) == 0 {
		return "", 0
	}
	u := h.urls[h.pos]
	return u.url, u.scrollPos
}

func (h *History) Forward() (string, int, bool) {
	h.Lock()
	defer h.Unlock()
	if h.pos < len(h.urls)-1 {
		h.pos++
		u := h.urls[h.pos]
		return u.url, u.scrollPos, true
	}
	return "", 0, false
}

func (h *History) Status() string {
	h.Lock()
	defer h.Unlock()
	return fmt.Sprintf("Count=%d, Pos=%d", len(h.urls), h.pos)
}

type jsonURL struct {
	URL       string
	ScrollPos int
}

type jsonData struct {
	URLs []jsonURL
	Pos  int
}

func (h *History) ToJSON(out io.Writer) error {
	h.Lock()
	defer h.Unlock()
	j := jsonData{Pos: h.pos}
	for _, u := range h.urls {
		j.URLs = append(j.URLs, jsonURL{u.url, u.scrollPos})
	}
	return json.NewEncoder(out).Encode(&j)
}

func FromJSON(in io.Reader) ([]*History, error) {
	dec := json.NewDecoder(in)
	var hs []*History
	for {
		var j jsonData
		if err := dec.Decode(&j); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		h := &History{pos: j.Pos}
		for _, u := range j.URLs {
			h.urls = append(h.urls, URL{u.URL, u.ScrollPos})
		}
		hs = append(hs, h)
	}
	return hs, nil
}
