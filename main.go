package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/rohenaz/go-bmap-indexer/crawler"
	"github.com/rohenaz/go-bmap-indexer/state"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
}

func main() {

	currentBlock := state.LoadProgress()

	go crawler.ProcessDone()
	crawler.SyncBlocks(int(currentBlock))

	<-make(chan struct{})
}
