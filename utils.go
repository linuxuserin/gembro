package main

import (
	"fmt"
	"log"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var once sync.Once

func getExt(mediaType string) string {
	once.Do(func() {
		if err := mime.AddExtensionType(".gmi", "text/gemini"); err != nil {
			log.Print(err)
		}
	})
	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		log.Print(err)
	}
	if len(exts) > 0 {
		return exts[len(exts)-1]
	}
	return ""
}

func suggestDownloadPath(title, url, mediaType string) string {
	hpath, _ := os.UserHomeDir()
	downloadDir := filepath.Join(hpath, "Downloads")
	if _, err := os.Stat(downloadDir); err == nil { // Dir exists
		hpath = downloadDir
	}
	name := title
	if name == "" {
		name = path.Base(url)
	}
	var ext string
	if i := strings.LastIndex(name, "."); i != -1 {
		ext = name[i:]
		name = name[:i]
	} else {
		ext = getExt(mediaType)
	}
	name = strings.NewReplacer(" ", "_", ".", "_").Replace(name)

	var extra string
	var count int
	for {
		newpath := filepath.Join(hpath, name+extra+ext)
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

func osOpenURL(url string) error {
	var opener string
	switch runtime.GOOS {
	case "windows":
		opener = "start"
	case "darwin":
		opener = "open"
	case "linux":
		fallthrough
	default:
		opener = "xdg-open"
	}

	err := exec.Command(opener, url).Start()
	if err != nil {
		return fmt.Errorf("could not open URL: %w", err)
	}
	return nil
}
