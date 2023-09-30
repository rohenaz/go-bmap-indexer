package main

import (
	"fmt"
	"log"
	"time"

	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/crawler"
	"github.com/rohenaz/go-bmap-indexer/persist"
	"github.com/rohenaz/go-bmap-indexer/state"
)

func syncWorker(currentBlock int, wait bool) {
	if wait {
		time.Sleep(30 * time.Second)
	}

	// crawl
	newBlock := crawler.SyncBlocks(currentBlock)

	// if we've indexed some new txs to bring into the state
	if newBlock > currentBlock {
		newBlock = state.SyncState(currentBlock)
	} else {
		fmt.Println("everything up-to-date")
	}

	go syncWorker(newBlock, true)
}

func main() {
	var currentBlock int

	// load persisted block to continue from
	if err := persist.Load("./block.tmp", &currentBlock); err != nil {
		log.Println(err, "Starting from default block.")
		currentBlock = config.FromBlock
	}

	// Create a channel for ready files
	readyFiles := make(chan string, 1000) // Adjust buffer size as needed

	// Start file watcher and worker
	go crawler.WatchFiles(readyFiles)
	go crawler.Worker(readyFiles)

	// Continue with your existing code
	syncWorker(currentBlock, false)
}
