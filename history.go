package main

import "fmt"

type History struct {
	urls []string
	pos  int
}

func (h *History) Add(surl string) {
	if len(h.urls) == 0 && h.pos == 0 {
		h.pos = -1
	}
	h.urls = append(h.urls[:h.pos+1], surl)
	h.pos = len(h.urls) - 1
}

func (h *History) Back() (string, bool) {
	if h.pos > 0 {
		h.pos--
		return h.urls[h.pos], true
	}
	return "", false
}

func (h *History) Forward() (string, bool) {
	if h.pos < len(h.urls)-1 {
		h.pos++
		return h.urls[h.pos], true
	}
	return "", false
}

func (h *History) Status() string {
	return fmt.Sprintf("Count=%d, Pos=%d", len(h.urls), h.pos)
}
