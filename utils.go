package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func suggestDownloadPath(name string) string {
	path, _ := os.UserHomeDir()
	downloadDir := filepath.Join(path, "Downloads")
	if _, err := os.Stat(downloadDir); err == nil { // Dir exists
		path = downloadDir
	}
	name = strings.NewReplacer(" ", "_", ".", "_").Replace(name)
	var extra string
	var count int
	for {
		newpath := filepath.Join(path, name+extra+".gmi")
		_, err := os.Stat(newpath)
		if os.IsNotExist(err) { // Not exists (or some other error)
			return newpath
		}
		count++
		if count > 100 { // Can't find available path, just suggest this one
			return newpath
		}
		extra = fmt.Sprintf("_%d", count)
	}
}