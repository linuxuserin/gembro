package bookmark

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Bookmark struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type BookmarkStore struct {
	sync.Mutex
	bookmarks []Bookmark
	path      string
}

func (bs *BookmarkStore) Add(surl, name string) error {
	bs.Lock()
	defer bs.Unlock()
	bs.bookmarks = append(bs.bookmarks, Bookmark{surl, name})
	return bs.save()
}

func (bs *BookmarkStore) Remove(surl string) error {
	bs.Lock()
	defer bs.Unlock()
	var newb []Bookmark
	for _, b := range bs.bookmarks {
		if b.URL != surl {
			newb = append(newb, b)
		}
	}
	bs.bookmarks = newb
	return bs.save()
}

func (bs *BookmarkStore) Contains(surl string) bool {
	bs.Lock()
	defer bs.Unlock()
	for _, b := range bs.bookmarks {
		if b.URL == surl {
			return true
		}
	}
	return false
}

func (bs *BookmarkStore) All() []Bookmark {
	return bs.bookmarks
}

type jsonBookmarks struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

func (bs *BookmarkStore) save() error {
	f, err := os.Create(bs.path)
	if err != nil {
		return fmt.Errorf("could not save bookmarks: %w", err)
	}
	defer f.Close()
	bookmarks := jsonBookmarks{bs.bookmarks}
	if err := json.NewEncoder(f).Encode(&bookmarks); err != nil {
		return fmt.Errorf("could not encode bookmarks: %w", err)
	}
	return nil
}

func Load(path string) (*BookmarkStore, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BookmarkStore{path: path}, nil
		}
		return nil, fmt.Errorf("could not open bookmarks file: %w", err)
	}
	var bookmarks jsonBookmarks
	if err := json.NewDecoder(f).Decode(&bookmarks); err != nil {
		return nil, fmt.Errorf("could not decode bookmarks: %w", err)
	}
	return &BookmarkStore{
		path:      path,
		bookmarks: bookmarks.Bookmarks,
	}, nil
}
