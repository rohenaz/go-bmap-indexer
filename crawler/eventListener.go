package crawler

import (
	"log"

	"github.com/GorillaPool/go-junglebus"
	"github.com/ttacon/chalk"
)

// func getWgForBlock(height int) *sync.WaitGroup {
// 	if wgs[uint32(height)] == nil {
// 		return &sync.WaitGroup{}
// 	}

// 	return wgs[uint32(height)]
// }

// var limiter = make(chan struct{}, 32)

// map of block height to tx count
var blocksDone = make(chan map[uint32]uint32, 1000)

func eventListener(subscription *junglebus.Subscription) {
	// var crawlHeight uint32
	// var wg sync.WaitGroup
	for event := range eventChannel {
		switch event.Type {
		case "transaction":
			processTransactionEvent(event.Transaction, event.Height, event.Time)

		case "status":
			switch event.Status {
			case "connected":
				log.Printf("%sConnected to Junglebus%s\n", chalk.Green, chalk.Reset)

				continue
			case "block-done":
				// wg.Wait()
				blocksDone <- map[uint32]uint32{event.Height: 0}
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
	for height := range blocksDone {
		processBlockDoneEvent(height)
	}
}
