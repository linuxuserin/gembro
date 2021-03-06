package text

type Link struct {
	URL, Name string
}

type Links struct {
	links map[int]Link
}

func (l *Links) Add(ypos int, url, name string) {
	if l.links == nil {
		l.links = make(map[int]Link)
	}
	l.links[ypos] = Link{url, name}
}

func (l Links) LinkAt(y int) *Link {
	if val, ok := l.links[y]; ok {
		return &val
	}
	return nil
}
