package crawler

import (
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

// WatchFiles for changes in the data directory
func WatchFiles(readyFiles chan string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start watching the data directory
	err = watcher.Add("data")
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event := <-watcher.Events:
			// Check if the file is ready for processing
			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				fileInfo, err := os.Stat(event.Name)
				if err != nil {
					log.Printf("Error stating file %s: %v", event.Name, err)
					continue
				}

				// Check if the file is read-only
				if fileInfo.Mode().Perm()&0222 == 0 {
					readyFiles <- event.Name
				}
			}
		case err := <-watcher.Errors:
			log.Println("Watcher error:", err)
		}
	}
}
