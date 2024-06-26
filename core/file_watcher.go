package core

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"regexp"
	"time"

	"path/filepath"

	"github.com/khelechy/argus/enums"
	"github.com/khelechy/argus/models"

	"github.com/fsnotify/fsnotify"
)

var messageChan chan string

func Watch(watchStructures []models.WatchStructure) {

	messageChan = make(chan string)

	// setup watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
		return
	}

	defer watcher.Close()

	done := make(chan bool)

	// use goroutine to start the watcher
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				newEvent := &models.Event{}
				newEvent.Timestamp = time.Now()
				newEvent.Name = event.Name

				if event.Op&fsnotify.Create == fsnotify.Create {
					newEvent.ActionDescription = fmt.Sprintf("File created: %s", event.Name)
					newEvent.Action = enums.Create
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					newEvent.ActionDescription = fmt.Sprintf("File modified: %s", event.Name)
					newEvent.Action = enums.Write
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					newEvent.ActionDescription = fmt.Sprintf("File removed: %s", event.Name)
					newEvent.Action = enums.Delete
				}
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					newEvent.ActionDescription = fmt.Sprintf("File renamed: %s", event.Name)
					newEvent.Action = enums.Rename
				}
				if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					newEvent.ActionDescription = fmt.Sprintf("File permissions modified: %s", event.Name)
					newEvent.Action = enums.Chmod
				}
				go func(transferredEvent models.Event) {
					data, err := json.Marshal(transferredEvent)
					if err != nil {
						log.Fatal(err)
						return
					}
					messageChan <- string(data)
				}(*newEvent)

			case err := <-watcher.Errors:
				log.Println("Error:", err)
			}
		}
	}()

	for _, watchStructure := range watchStructures {
		if watchStructure.WatchRecursively { // Watch Recursively
			if err := filepath.Walk(watchStructure.Path, func(path string, fi os.FileInfo, err error) error {
				if _, err := os.Stat(path); err != nil {
					return err
				}

				// since fsnotify can watch all the files in a directory, watchers only need
				// to be added to each nested directory
				if fi.Mode().IsDir() {
					return watcher.Add(path)
				}

				if err := watcher.Add(path); err != nil {
					log.Println("ERROR", err)
				} // since path isnt a directory, add to watch (non recursively)

				return nil
			}); err != nil {
				log.Println("ERROR", err)
			}
		} else {

			err = watcher.Add(watchStructure.Path)
			if err != nil {
				log.Println("ERROR", err)
			}
		}
	}

	<-done
}

func TestForWildCard(path string) (bool, []models.WatchStructure) {
	regex := regexp.MustCompile(`^(.*)\*\.(\w+)`)
	matches := regex.FindStringSubmatch(path)

	if len(matches) == 0 {
		return false, nil
	}

	dir := matches[1]
	ext := matches[2]

	pattern := fmt.Sprintf("%v\\w+\\.%v", dir, ext)
	pathregex := regexp.MustCompile(pattern)

	var matchedFiles []models.WatchStructure = []models.WatchStructure{}

	err := filepath.Walk(dir, func(pathName string, fi fs.FileInfo, err error) error {
		if !fi.IsDir() {
			if pathregex.Match([]byte(pathName)) {
				matchedFiles = append(matchedFiles, models.WatchStructure{
					Path:             pathName,
					WatchRecursively: false,
				})
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error: %v", err.Error())
		return false, nil
	}

	return true, matchedFiles
}

func PrepWatchList(watchlist []models.WatchStructure) []models.WatchStructure {
	var out []models.WatchStructure = []models.WatchStructure{}

	for _, structure := range watchlist {
		isWild, list := TestForWildCard(structure.Path)
		if isWild {
			out = append(out, list...)
		} else {
			out = append(out, structure)
		}
	}
	return out
}
