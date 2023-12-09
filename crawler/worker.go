package crawler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/database"
	"github.com/ttacon/chalk"
	"go.mongodb.org/mongo-driver/bson"
)

var CONCURRENT_INSERTS = 32

// Worker for processing files
func Worker(readyFiles chan string) {
	for filename := range readyFiles {
		// Process the file
		ingest(filename)

		// After successful import, delete the file
		// in p2p mode, its deleted after saving to redis
		if config.DeleteAfterIngest && !config.EnableP2P {
			fmt.Printf("%sDeleting file in crawler worker %s%s\n", chalk.Cyan, filename, chalk.Reset)
			err := os.Remove(filename)
			if err != nil {
				fmt.Printf("%s%s %s: %v%s\n", chalk.Cyan, "Error deleting file", filename, err, chalk.Reset)
			}
		}
	}
}

// ingest JSONLD file and ingest each line as a mongo document
func ingest(filepath string) {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		log.Panicf("%s%s %s: %v%s\n", chalk.Cyan, "Error opening file", filepath, err, chalk.Reset)
		return
	}
	defer file.Close()

	// Create a new Scanner for the file
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // set the buffer to 10MB

	var wg sync.WaitGroup
	limiter := make(chan struct{}, CONCURRENT_INSERTS)
	for scanner.Scan() {
		// 1 - read each string line from the file path
		line := scanner.Text()

		// 2 - unmarshal into bmap
		var bsonData bson.M
		byteLine := []byte(line)
		err := json.Unmarshal(byteLine, &bsonData)
		if err != nil {
			log.Panicf("%s[Error]: %s%s\n", chalk.Cyan, err, chalk.Reset)
			continue
		}

		// 2.1 - get the collection name
		// panic: interface conversion: interface {} is []interface {}, not primitive.M
		collectionName, ok := bsonData["MAP"].([]interface{})[0].(map[string]interface{})["type"].(string)
		if !ok {
			log.Panicf("%s[Error]: %s%s\n", chalk.Cyan, "Could not get collection name", chalk.Reset)
			continue
		}
		// 2.5 find existing record in the db
		existing, err := GetExistingDoc(collectionName, bsonData["_id"].(string))
		if err != nil {
			log.Panicf("%s[Error]: %s%s\n", chalk.Cyan, err, chalk.Reset)
			continue
		}
		if existing != nil && existing.Timestamp == 0 {
			// update the timestamp
			bsonData["timestamp"] = existing.Timestamp
		}
		limiter <- struct{}{}
		wg.Add(1)
		go func(bsonData *bson.M) {
			defer func() {
				<-limiter
				wg.Done()
			}()
			// 3 - insert into mongo
			err = saveToMongo(bsonData)
			if err != nil {
				log.Panicf("%s[Error]: %s%s\n", chalk.Cyan, err, chalk.Reset)
			}
		}(&bsonData)
	}

	wg.Wait()

	// Check for errors in the scanner
	if err := scanner.Err(); err != nil {
		fmt.Printf("%sError reading file %s: %v%s\n", chalk.Cyan, filepath, err, chalk.Reset)
		return
	}
}

// GetExistingDoc returns a document from the txs collection
func GetExistingDoc(collectionName string, txid string) (*database.IndexerTx, error) {
	conn := database.GetConnection()
	filter := bson.M{"_id": txid}

	bmapTx, err := conn.GetDocs(collectionName, 1, 0, filter)
	if err != nil {
		return nil, err
	}
	if len(bmapTx) == 0 {
		return nil, nil
	}
	return &bmapTx[0], nil
}

func saveToMongo(bsonData *bson.M) (err error) {
	conn := database.GetConnection()
	// if len(bmapData.MAP) == 0 || len(bmapData.MAP[0]) == 0 {
	// 	return fmt.Errorf("No MAP data")
	// }
	// _, ok := bmapData.MAP[0]["app"].(string)
	// if !ok {
	// 	return fmt.Errorf("MAP 'app' key does not exist")
	// }

	// I'm getting an error that this is nil not a string
	// 	panic: interface conversion: interface {} is nil, not string

	// goroutine 33 [running]:
	// github.com/rohenaz/go-bmap-indexer/crawler.saveToMongo(0xc000560088)
	//         /Users/satchmo/code/go-bmap-indexer/crawler/worker.go:99 +0x165

	var collectionName string
	var ok bool
	if collectionName, ok = (*bsonData)["collection"].(string); !ok {
		return
	}
	delete(*bsonData, "collection")

	filter := bson.M{"_id": (*bsonData)["_id"]}

	// bsonData := bson.M{
	// 	"_id": filter["_id"],
	// 	"tx":  bmapData.Tx,
	// 	"blk": bmapData.Blk,
	// 	"MAP": bmapData.MAP,
	// }

	// if bmapData.AIP != nil {
	// 	bsonData["AIP"] = bmapData.AIP
	// }

	// if bmapData.B != nil {
	// 	bsonData["B"] = bmapData.B
	// }

	// log.Println("Inserting into collection", collectionName)
	_, err = conn.UpsertOne(collectionName, filter, *bsonData)

	return
}
