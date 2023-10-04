package state

import (
	"fmt"
	"log"
	"time"

	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/database"
	"go.mongodb.org/mongo-driver/bson"
)

// SaveProgress persists the block height to ./block.tmp
func SaveProgress(height uint32) {
	if height > 0 {

		// persist our progress to the database
		// TODO save height to _state collection
		// { _id: 'height', value: height }
		conn := database.GetConnection()

		_, err := conn.UpsertOne("_state", bson.M{"_id": "_state"}, bson.M{"height": height})
		if err != nil {
			log.Printf("[ERROR]: %v", err)
			return
		}
	}

}

// LoadProgress loads the block height from ./block.tmp
func LoadProgress() (height uint32) {

	// load height from _state collection

	conn := database.GetConnection()

	doc, err := conn.GetStateDocs("_state", 1, 0, bson.M{"_id": "_state"})
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return
	}

	if len(doc) == 0 {
		log.Printf("[ERROR]: No state found")

		// create initial state document
		conn.UpsertOne("_state", bson.M{"_id": "_state"}, bson.M{"height": uint32(config.FromBlock)})

		height = config.FromBlock
		return
	}

	// use the []primitive.M to get the height value
	height = uint32(doc[0]["height"].(int64))

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

	// set skipSpv to true to trust every tx exists on the blockchain,
	// false to verify every tx with a miner
	newBlock = build(fromBlock, config.SkipSPV)
	diff := time.Since(stateStart).Seconds()
	fmt.Printf("State sync complete to block height %d in %fs\n", newBlock, diff)

	// update the state block clounter
	SaveProgress(uint32(newBlock))

	return
}
