package text

type Link struct {
	URL, Name string
	index     int
}

type Links struct {
	links map[int]Link
}

func (l *Links) Add(ypos, index int, url, name string) {
	if l.links == nil {
		l.links = make(map[int]Link)
	}
	l.links[ypos] = Link{url, name, index}
}

func (l Links) LinkAt(y int) *Link {
	if val, ok := l.links[y]; ok {
		return &val
	}
	return nil
}

func (l Links) Number(n int) *Link {
	for _, l := range l.links {
		if l.index == n {
			return &l
		}
	}
	return nil
}

func (l *Links) Count() int {
	return len(l.links)
}
