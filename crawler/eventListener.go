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
var blocksDone = make(chan uint32, 1000)

func eventListener(subscription *junglebus.Subscription) {
	// var crawlHeight uint32
	// var wg sync.WaitGroup
	for event := range eventChannel {
		switch event.Type {
		case "transaction":
			processTransactionEvent(event.Transaction, event.Height, event.Time)

		case "status":
			switch event.Status {
			// case "connected":
			// 	log.Printf("%s[INFO]: Connected to JungleBus at block %d %s\n", chalk.Green, crawlHeight, chalk.Reset)
			// 	conn := database.GetConnection()
			// 	// get the number of retries from the db
			// 	doc, err := conn.GetStateDocs("_state", 1, 0, bson.M{"_id": "_state"})
			// 	if err != nil {
			// 		log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
			// 		return
			// 	}

			// 	if len(doc) == 0 {
			// 		log.Printf("%s[ERROR]: No state found%s\n", chalk.Green, chalk.Reset)
			// 		return
			// 	}

			// 	// Look up the attempted retries for this block
			// 	blockDoc, err := conn.GetStateDocs("_blocks", 1, 0, bson.M{"_id": crawlHeight})
			// 	if err != nil {
			// 		log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
			// 		continue
			// 	}
			// 	var retries int32
			// 	if len(blockDoc) == 0 {
			// 		retries = 0
			// 	} else {
			// 		// use the []primitive.M to get the retries value for this block height
			// 		retries = blockDoc[0]["retries"].(int32)
			// 	}

			// 	// if the counter is over the configured limit,
			// 	// by incrementing the hight
			// 	if retries >= config.BockSyncRetries {
			// 		log.Printf("%s[ERROR]: Max retries reached for block %d%s\n", chalk.Green, crawlHeight, chalk.Reset)

			// 		// move on to the next block
			// 		crawlHeight++

			// 		// wg := getWgForBlock(crawlHeight)
			// 		// if wg != nil {
			// 		// 	log.Printf("%sWaiting for wg...%s\n", chalk.Green, chalk.Reset)
			// 		// wg.Wait()
			// 		// }

			// 		CancelCrawl(crawlHeight)

			// 		return
			// 	} else {
			// 		// if the counter is under the configured limit,
			// 		// increment the reconnect counter
			// 		retries++
			// 		log.Printf("%sRetrying %d more times...%s\n", chalk.Green, config.BockSyncRetries-retries, chalk.Reset)
			// 	}
			// 	// insert a document to track the crawl state for each block
			// 	_, err = conn.UpsertOne("_blocks", bson.M{"_id": crawlHeight}, bson.M{"retries": retries, "height": crawlHeight})
			// 	if err != nil {
			// 		log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
			// 		continue
			// 	}

			case "block-done":
				// wg.Wait()
				blocksDone <- event.Height
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
