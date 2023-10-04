package crawler

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/ttacon/chalk"
)

// WatchFiles for changes in the data directory
func WatchFiles(readyFiles chan string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("%s%s%s%s\n", chalk.Red, "Error: ", err, chalk.Reset)
		return
	}
	defer watcher.Close()

	// Check if "data" directory exists
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		err := os.Mkdir("data", 0755)
		if err != nil {
			fmt.Printf("%s%s%s%s\n", chalk.Red, "Error creating data directory:", err, chalk.Reset)
			return
		}
	}

	// Start watching the data directory
	fmt.Println(chalk.Red, "Watching data folder...", chalk.Reset)

	err = watcher.Add("data")
	if err != nil {
		fmt.Printf("%s%s%s%s\n", chalk.Red, "Error: ", err, chalk.Reset)
		return
	}

	for {
		select {
		case event := <-watcher.Events:
			// Check if the file is ready for processing
			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				fileInfo, err := os.Stat(event.Name)
				if err != nil {
					fmt.Printf("%s%s%s: %v%s\n", chalk.Red, "Error getting file stats", event.Name, err, chalk.Reset)
					continue
				}

				// Check if the file is read-only
				if fileInfo.Mode().Perm()&0222 == 0 {
					readyFiles <- event.Name
				}
			}
		case err := <-watcher.Errors:
			fmt.Printf("%s%s%s%s\n", chalk.Red, "Watcher error:", err, chalk.Reset)
			return
		}
	}
}
