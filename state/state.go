package state

import (
	"fmt"
	"log"
	"time"

	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/database"
	"github.com/rohenaz/go-bmap-indexer/persist"
)

// SaveProgress persists the block height to ./block.tmp
func SaveProgress(height uint32) {
	if height > 0 {
		if config.UseDBForState {
			// persist our progress to the database
			// TODO save height to _state collection
			// { _id: 'height', value: height }
		} else {
			// persist our progress to disk
			if err := persist.Save("./block.tmp", height); err != nil {
				log.Fatalln(err)
			}
		}
	}

}

// LoadProgress loads the block height from ./block.tmp
func LoadProgress() (height uint32) {
	if config.UseDBForState {
		// TODO: load height from _state collection
		// { _id: 'height', value: height }
	} else {
		// load height from disk
		if err := persist.Load("./block.tmp", &height); err != nil {
			log.Println(err, "Starting from default block.")
			height = config.FromBlock
			// Create the file if it doesn't exist
			if err := persist.Save("./block.tmp", height); err != nil {
				log.Println("Error creating block.tmp:", err)
			}
		}
	}

	return
}

func build(fromBlock int, trust bool) (stateBlock int) {
	// if there are no txs to process, return the same thing we sent in
	stateBlock = fromBlock

	// var numPerPass int = 100

	// Query x records at a time in a loop
	// ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	conn := database.GetConnection()

	// defer conn.Disconnect(ctx)

	// Clear old state
	if fromBlock == 0 {
		log.Println("Clearing state")
		conn.ClearState()
	}

	// TODO: Implement state sync
	return stateBlock
}

func SyncState(fromBlock int) (newBlock int) {
	// Set up timer for state sync
	stateStart := time.Now()

	// set skipSpv to true to trust every tx exists on the blopckchain,
	// false to verify every tx with a miner
	newBlock = build(fromBlock, config.SkipSPV)
	diff := time.Now().Sub(stateStart).Seconds()
	fmt.Printf("State sync complete to block height %d in %fs\n", newBlock, diff)

	// update the state block clounter
	SaveProgress(uint32(newBlock))

	return
}
