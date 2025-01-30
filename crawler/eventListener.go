package crawler

import (
	"fmt"
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
			// log.Printf("%sTransaction %s %s\n", chalk.Green, event.Id, chalk.Reset)
			processTransactionEvent(event.Transaction, event.Height, event.Time)

		case "status":
			switch event.Status {
			case "disconnected":
				txCount = 0
				log.Fatalf("%sDisconnected from Junglebus. Reset tx counter.%s\n", chalk.Green, chalk.Reset)
			case "connected":
				log.Printf("%sConnected to Junglebus%s\n", chalk.Green, chalk.Reset)

				continue
			case "waiting":
				log.Printf("%sWaiting for new blocks%s\n", chalk.Green, chalk.Reset)
				// if config.EnableP2P && !p2p.Started {
				// 	// convert jsonld files to individual cbor files suitable for p2p transmission
				// 	p2p.CreateContentCache()
				// 	go p2p.Start()
				// }
				continue
			case "block-done":
				// copy the var
				var count = txCount
				if count > 0 {
					log.Printf("%sBlock %d done with %d transactions%s\n", chalk.Green, event.Height, count, chalk.Reset)
					blocksDone <- map[uint32]uint32{event.Height: count}
				}
				txCount = 0
				continue
			}
		case "mempool":
			_, _, err := processMempoolEvent(event.Transaction)
			if err != nil {
				fmt.Printf("%s%s%s\n", chalk.Red, err.Error(), chalk.Reset)
				continue
			}
		case "error":
			log.Printf("%sERROR: %s%s\n", chalk.Green, event.Error.Error(), chalk.Reset)
		}
	}
}

func ProcessDone() {
	for heightMap := range blocksDone {
		// loop over single entry map
		for height, txCount := range heightMap {
			if txCount > 0 {
				processBlockDoneEvent(height, txCount)
				//if config.EnableP2P {
				// p2p.CreateContentCache()
				//}
			}
			break
		}
	}
}
