package crawler

import (
	"context"
	"log"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"fmt"

	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	"github.com/bitcoinschema/go-bmap"
	"github.com/libsv/go-bt/v2"
	"github.com/mitchellh/copystructure"
	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/persist"
	"github.com/rohenaz/go-bmap-indexer/state"
	"go.mongodb.org/mongo-driver/bson"
)

var wg sync.WaitGroup

func SyncBlocks(height int) (newBlock int) {
	// Setup crawl timer
	crawlStart := time.Now()

	// Crawl will mutate currentBlock
	newBlock = Crawl(height)

	// Crawl complete
	diff := time.Since(crawlStart).Seconds()

	// TODO: I believe if we get here crawl has actually died
	fmt.Printf("Bitbus sync complete in %fs\nBlock height: %d\n", diff, height)
	return
}

type Event struct {
	Type        string
	Transaction *models.TransactionResponse
	Status      *models.ControlResponse
	Error       error
}

// Crawl loops over the new bmap transactions since the given block height
func Crawl(height int) (newHeight int) {

	eventChannel := make(chan Event, 1000) // Buffered channel

	junglebusClient, err := junglebus.New(
		junglebus.WithHTTP("https://junglebus.gorillapool.io"),
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	subscriptionID := config.SubscriptionID

	// get from block from block.tmp
	fromBlock := uint64(config.FromBlock)

	lastBlock := uint64(state.LoadProgress())
	if lastBlock > fromBlock {
		fromBlock = lastBlock
	}

	eventHandler := junglebus.EventHandler{
		// Mined tx callback
		OnTransaction: func(tx *models.TransactionResponse) {
			log.Printf("[TXa]: %d: %v", tx.BlockHeight, tx.Id)

			transactionCopy, err := copystructure.Copy(tx)
			if err != nil {
				log.Printf("ERROR: %s", err.Error())
			} else {
				if txCopy, ok := transactionCopy.(*models.TransactionResponse); ok {
					eventChannel <- Event{Type: "transaction", Transaction: txCopy}
				} else {
					log.Printf("ERROR: Failed to assert transactionCopy to *models.TransactionResponse")
				}
			}

		},
		// Mempool tx callback
		OnMempool: func(tx *models.TransactionResponse) {
			log.Printf("[MEMa]: %d: %v", tx.BlockHeight, tx.Id)

			eventChannel <- Event{Type: "mempool", Transaction: tx}
		},
		OnStatus: func(status *models.ControlResponse) {
			log.Printf("[STATa]: %d: %v", status.Block, status.Status)

			eventChannel <- Event{Type: "status", Status: status}
		},
		OnError: func(err error) {
			eventChannel <- Event{Type: "error", Error: err}
		},
	}

	fmt.Printf("Initializing from block %d\n", fromBlock)

	var subscription *junglebus.Subscription
	if subscription, err = junglebusClient.Subscribe(context.Background(), subscriptionID, fromBlock, eventHandler); err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
	}

	if err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
		unsubscribeError := subscription.Unsubscribe()

		if err = subscription.Unsubscribe(); unsubscribeError != nil {
			log.Printf("ERROR: failed unsubscribing %s", err.Error())
		}
	}

	go func() {
		for event := range eventChannel {
			switch event.Type {
			case "transaction":
				if event.Transaction != nil {
					wg.Add(1)
					processTransactionEvent(event.Transaction)
					wg.Done()
				} else {
					log.Printf("ERROR: Transaction is nil!!")
				}
			case "status":
				if event.Status.Status == "block-done" {
					wg.Wait()
					processBlockDoneEvent(event.Status, height)
				}
			case "mempool":
				processMempoolEvent(event.Transaction)
			case "error":
				log.Printf("ERROR: %s", event.Error.Error())
			}
		}
	}()

	// wait indefinitely to make sure we dont stop
	// before more mempool txs come in
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()

	// Print tx line to stdout
	if err != nil {
		fmt.Println(err)
	}

	return
}

func processTransactionEvent(tx *models.TransactionResponse) {
	if tx != nil && tx.Id != "" {

		if len(tx.Transaction) != 0 {
			// log.Println("Processing txb...")
			// Write to disk

			if len(tx.Transaction) > 0 {

				t, err := bt.NewTxFromBytes(tx.Transaction)
				if err != nil {
					log.Printf("[ERROR]: %v", err)
					return
				}
				bmapTx, err := bmap.NewFromTx(t)
				if err != nil {
					log.Printf("[ERROR]: %v", err)
					return
				}

				bmapTx.Blk.I = tx.BlockHeight
				bmapTx.Blk.T = tx.BlockTime

				log.Printf("[BMAP]: %d: %s | Data Length: %d | First 10 bytes: %x", tx.BlockHeight, bmapTx.Tx.Tx.H, len(tx.Transaction), tx.Transaction[:10])

				processTx(bmapTx)
			} else {
				log.Printf("ERROR: Transaction is nil 3!!")
			}
		} else {
			log.Printf("ERROR: Transaction is nil 2!!")
		}
	} else {
		log.Printf("ERROR: Transaction is nil 4!!")
	}
}

func processMempoolEvent(tx *models.TransactionResponse) {
	log.Printf("[MEMPOOL TX]: %s", tx.Id)
	// t, err := bt.NewTxFromBytes(tx.Transaction)
	// if err != nil {
	// 	log.Printf("[ERROR]: %v", err)
	// 	return
	// }
	// bmapTx, err := bmap.NewFromTx(t)
	// if err != nil {
	// 	log.Printf("[ERROR]: %v", err)
	// 	return
	// }
	// log.Printf("[MEMPOOL BMAP]: %d: %v", bmapTx.Blk.I, bmapTx.Tx.Tx.H)
}

func processBlockDoneEvent(status *models.ControlResponse, height int) {
	// status = "block-done"
	log.Printf("[STATUS]: %v, %d", status, height)
	filename := fmt.Sprintf("data/%d.json", status.Block)

	if int(status.Block) > height {
		height = int(status.Block)
		state.SaveProgress(uint32(height))
	}

	// check if the file exists at path
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Block done with %d txs", 0)
		return
	}

	// change file to readonly (ready for ingestion)
	err := os.Chmod(filename, 0444) // read-only permissions
	if err != nil {
		log.Printf("Error changing permissions for %s: %v", filename, err)
	}

}

func processTx(bmapData *bmap.Tx) {
	bsonData := bson.M{
		"_id": bmapData.Tx.Tx.H,
		"tx":  bmapData.Tx.Tx,
		"blk": bmapData.Tx.Blk,
	}

	if bmapData.AIP != nil {
		bsonData["AIP"] = bmapData.AIP
	}

	if bmapData.BAP != nil {
		bsonData["BAP"] = bmapData.BAP
	}

	if bmapData.Ord != nil {
		bsonData["Ord"] = bmapData.Ord
	}

	if bmapData.B != nil {
		bsonData["B"] = bmapData.B
	}

	if bmapData.BOOST != nil {
		bsonData["BOOST"] = bmapData.BOOST
	}

	if bmapData.MAP == nil {
		log.Println("No MAP data.")
		return
	}
	bsonData["MAP"] = bmapData.MAP
	if _, ok := bmapData.MAP[0]["type"].(string); !ok {
		log.Println("Error: MAP 'type' key does not exist.")
		return
	}
	if _, ok := bmapData.MAP[0]["app"].(string); !ok {
		log.Println("Error: MAP 'app' key does not exist.")
		return
	}

	for key, value := range bsonData {
		if str, ok := value.(string); ok {
			if !utf8.ValidString(str) {
				log.Printf("Invalid UTF-8 detected in key %s: %s", key, str)
				return
			}
		}
	}

	// 	Write to local filesystem
	err := persist.SaveLine(fmt.Sprintf("data/%d.json", bmapData.Blk.I), bsonData)
	if err != nil {
		log.Printf("[WRITE ERROR]: %v", err)
		return
	}
}
