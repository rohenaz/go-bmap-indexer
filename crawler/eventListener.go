package crawler

import (
	"log"

	"github.com/GorillaPool/go-junglebus"
	"github.com/ttacon/chalk"
)

// map of block height to tx count
var blocksDone = make(chan map[uint32]uint32, 1000)

var txCount uint32

func eventListener(subscription *junglebus.Subscription) {
	// var crawlHeight uint32
	// var wg sync.WaitGroup
	for event := range eventChannel {
		switch event.Type {
		case "transaction":
			txCount++
			processTransactionEvent(event.Transaction, event.Height, event.Time)

		case "status":
			switch event.Status {
			case "connected":
				log.Printf("%sConnected to Junglebus%s\n", chalk.Green, chalk.Reset)

				continue
			case "block-done":
				// Convert a string to a uint32

				log.Printf("%sBlock %d done%s\n", chalk.Green, event.Height, chalk.Reset)
				blocksDone <- map[uint32]uint32{event.Height: txCount}
				txCount = 0
				continue
			}
		case "mempool":
			// processMempoolEvent(event.Transaction)
		case "error":
			log.Printf("%sERROR: %s%s\n", chalk.Green, event.Error.Error(), chalk.Reset)
		}
	}
}

func ProcessDone() {
	for heightMap := range blocksDone {
		// loop over single entry map
		for height, txCount := range heightMap {
			processBlockDoneEvent(height, txCount)
			break
		}
	}
}
