package history

import (
	"fmt"
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
