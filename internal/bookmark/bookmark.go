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

type Store struct {
	sync.Mutex
	bookmarks []Bookmark
	path      string
}

func (bs *Store) Add(surl, name string) error {
	bs.Lock()
	defer bs.Unlock()
	bs.bookmarks = append(bs.bookmarks, Bookmark{surl, name})
	return bs.save()
}

func (bs *Store) Remove(surl string) error {
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

func (bs *Store) Contains(surl string) bool {
	bs.Lock()
	defer bs.Unlock()
	for _, b := range bs.bookmarks {
		if b.URL == surl {
			return true
		}
	}
	return false
}

func (bs *Store) All() []Bookmark {
	return bs.bookmarks
}

type jsonBookmarks struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

func (bs *Store) save() error {
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

func Load(path string) (*Store, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{path: path}, nil
		}
		return nil, fmt.Errorf("could not open bookmarks file: %w", err)
	}
	var bookmarks jsonBookmarks
	if err := json.NewDecoder(f).Decode(&bookmarks); err != nil {
		return nil, fmt.Errorf("could not decode bookmarks: %w", err)
	}
	return &Store{
		path:      path,
		bookmarks: bookmarks.Bookmarks,
	}, nil
}
