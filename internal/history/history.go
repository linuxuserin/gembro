package history

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type History struct {
	sync.Mutex
	urls []string
	pos  int
}

func (h *History) Add(surl string) {
	h.Lock()
	defer h.Unlock()
	if len(h.urls) == 0 && h.pos == 0 {
		h.pos = -1
	}
	h.urls = append(h.urls[:h.pos+1], surl)
	h.pos = len(h.urls) - 1
}

func (h *History) Back() (string, bool) {
	h.Lock()
	defer h.Unlock()
	if h.pos > 0 {
		h.pos--
		return h.urls[h.pos], true
	}
	return "", false
}

func (h *History) Current() string {
	h.Lock()
	defer h.Unlock()
	if len(h.urls) == 0 {
		return ""
	}
	return h.urls[h.pos]
}

func (h *History) Forward() (string, bool) {
	h.Lock()
	defer h.Unlock()
	if h.pos < len(h.urls)-1 {
		h.pos++
		return h.urls[h.pos], true
	}
	return "", false
}

func (h *History) Status() string {
	h.Lock()
	defer h.Unlock()
	return fmt.Sprintf("Count=%d, Pos=%d", len(h.urls), h.pos)
}

type jsonData struct {
	URLs []string
	Pos  int
}

func (h *History) ToJSON(out io.Writer) error {
	h.Lock()
	defer h.Unlock()
	j := jsonData{h.urls, h.pos}
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
		hs = append(hs, &History{urls: j.URLs, pos: j.Pos})
	}
	return hs, nil
}
