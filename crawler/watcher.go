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

	// Check if "data" directory exists
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		err := os.Mkdir("data", 0755)
		if err != nil {
			log.Fatal("Error creating data directory:", err)
		}
	}

	// Start watching the data directory
	log.Println("Watching data folder...")
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
