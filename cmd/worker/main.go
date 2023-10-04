package main

import (
	"log"
	"sync"
	"time"

	"github.com/rohenaz/go-bmap-indexer/crawler"
	"github.com/rohenaz/go-bmap-indexer/state"
)

func syncWorker(currentBlock int, wait bool) {
	if wait {
		log.Println("CALLING WORKER in 3 seconds")
		time.Sleep(3 * time.Second)
	}

	// crawl
	newBlock := crawler.SyncBlocks(currentBlock)

	// TODO: Enable sync state
	// for adding things like BAP identities
	// if we've indexed some new txs to bring into the state
	// if newBlock > currentBlock {
	// 	newBlock = state.SyncState(currentBlock)
	// } else {
	// 	fmt.Println("everything up-to-date")
	// }
	go syncWorker(newBlock, true)
}

func main() {

	currentBlock := state.LoadProgress()
	// load persisted block to continue from
	// if err := persist.Load("./block.tmp", &currentBlock); err != nil {
	// 	log.Println(err, "Starting from default block.")
	// 	currentBlock = config.FromBlock
	// }

	// Create a channel for ready files
	readyFiles := make(chan string, 1000) // Adjust buffer size as needed

	// Watch files and trigger events when files are ready to ingest (read only)
	go crawler.WatchFiles(readyFiles)

	// Listen for events and ingest transactions
	go crawler.Worker(readyFiles)

	// Subscribes to JungleBus
	// and appends transactions to JSON LD files per block in the ./data directory
	// mark files readonly when they are ready for ingestion
	syncWorker(int(currentBlock), false)

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
