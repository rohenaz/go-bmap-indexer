package crawler

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/GorillaPool/go-junglebus"
	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/database"
	"github.com/ttacon/chalk"
	"go.mongodb.org/mongo-driver/bson"
)

func getWgForBlock(height int) *sync.WaitGroup {
	if wgs[uint32(height)] == nil {
		return &sync.WaitGroup{}
	}

	return wgs[uint32(height)]
}

func eventListener(height int, subscription *junglebus.Subscription) {
	crawlHeight := height
	for event := range eventChannel {
		switch event.Type {
		case "transaction":
			if event.Transaction != nil {
				wg := wgs[event.Transaction.BlockHeight]
				if wg == nil {
					// first tx for this block, create a new wg
					wgs[event.Transaction.BlockHeight] = &sync.WaitGroup{}
					wg = wgs[event.Transaction.BlockHeight]
				}
				wg.Add(1)
				processTransactionEvent(event.Transaction)
				wg.Done()
			} else {
				log.Printf("%sERROR: Transaction is nil!!%s\n", chalk.Green, chalk.Reset)
			}
		case "status":
			switch event.Status.Status {
			case "connected":
				log.Printf("%s[INFO]: Connected to JungleBus at block %d %s\n", chalk.Green, crawlHeight, chalk.Reset)
				conn := database.GetConnection()
				// get the number of retries from the db
				doc, err := conn.GetStateDocs("_state", 1, 0, bson.M{"_id": "_state"})
				if err != nil {
					log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
					return
				}

				if len(doc) == 0 {
					log.Printf("%s[ERROR]: No state found%s\n", chalk.Green, chalk.Reset)
					return
				}

				// Look up the attempted retries for this block
				blockDoc, err := conn.GetStateDocs("_blocks", 1, 0, bson.M{"_id": crawlHeight})
				if err != nil {
					log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
					continue
				}
				var retries int32
				if len(blockDoc) == 0 {
					retries = 0
				} else {
					// use the []primitive.M to get the retries value for this block height
					retries = blockDoc[0]["retries"].(int32)
				}

				// if the counter is over the configured limit,
				// by incrementing the hight
				if retries >= config.BockSyncRetries {
					log.Printf("%s[ERROR]: Max retries reached for block %d%s\n", chalk.Green, crawlHeight, chalk.Reset)

					// move on to the next block
					crawlHeight++

					wg := getWgForBlock(crawlHeight)
					if wg != nil {
						log.Printf("%sWaiting for wg...%s\n", chalk.Green, chalk.Reset)
						wg.Wait()
					}

					CancelCrawl(crawlHeight)

					continue
				} else {
					// if the counter is under the configured limit,
					// increment the reconnect counter
					retries++
					log.Printf("%sRetrying %d more times...%s\n", chalk.Green, config.BockSyncRetries-retries, chalk.Reset)
				}
				// insert a document to track the crawl state for each block
				_, err = conn.UpsertOne("_blocks", bson.M{"_id": crawlHeight}, bson.M{"retries": retries, "height": crawlHeight})
				if err != nil {
					log.Printf("%s[ERROR]: %v%s\n", chalk.Green, err, chalk.Reset)
					continue
				}
			}
		case "block-done":

			// we keep seperrate waitgroups per block so new txs for other blocks
			// cannot increment our wg before its been flushed
			wg := wgs[event.Transaction.BlockHeight]
			if wg != nil {
				log.Printf("%sWaiting for wg...%s\n", chalk.Green, chalk.Reset)
				wg.Wait()
				log.Printf("%swg flushed: %v, %d%s\n", chalk.Green, event.Status, height, chalk.Reset)
			} else {

				log.Printf("%swg Nothing to flush: %v, %d%s\n", chalk.Green, event.Status, height, chalk.Reset)
			}

			if event.Status.StatusCode == 200 && event.Status.Block == 0 {
				fmt.Printf("%sCrawler Reset!!!! Unsubscribing and exiting...%s\n", chalk.Green, chalk.Reset)
				subscription.Unsubscribe()
				os.Exit(0)
			}
			processBlockDoneEvent(event.Status, height)
		case "mempool":
			processMempoolEvent(event.Transaction)
		case "error":
			log.Printf("%sERROR: %s%s\n", chalk.Green, event.Error.Error(), chalk.Reset)
		}
	}
}
