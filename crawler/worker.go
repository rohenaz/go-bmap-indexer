package crawler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bitcoinschema/go-bmap"
	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/database"
	"github.com/ttacon/chalk"
	"go.mongodb.org/mongo-driver/bson"
)

// Worker for processing files
func Worker(readyFiles chan string) {
	for filename := range readyFiles {
		// Process the file
		ingest(filename)

		// After successful import, delete the file
		if config.DeleteAfterIngest {
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
		fmt.Printf("%s%s %s: %v%s\n", chalk.Cyan, "Error opening file", filepath, err, chalk.Reset)
		return
	}
	defer file.Close()

	// Create a new Scanner for the file
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // set the buffer to 10MB

	for scanner.Scan() {
		// 1 - read each string line from the file path
		line := scanner.Text()

		// 2 - unmarshal into bmap
		var bmapData bmap.Tx
		byteLine := []byte(line)
		err := json.Unmarshal(byteLine, &bmapData)
		if err != nil {
			fmt.Printf("%s[Error]: %s%s\n", chalk.Cyan, err, chalk.Reset)
			continue
		}

		// 3 - insert into mongo
		err = saveToMongo(&bmapData)
		if err != nil {
			log.Panicf("%s[Error]: %s%s\n", chalk.Cyan, err, chalk.Reset)

			continue
		}
	}

	// Check for errors in the scanner
	if err := scanner.Err(); err != nil {
		fmt.Printf("%sError reading file %s: %v%s\n", chalk.Cyan, filepath, err, chalk.Reset)
		return
	}
}

func saveToMongo(bmapData *bmap.Tx) (err error) {
	conn := database.GetConnection()
	if len(bmapData.MAP[0]) == 0 {
		return fmt.Errorf("No MAP data")
	}
	_, ok := bmapData.MAP[0]["app"].(string)
	if !ok {
		return fmt.Errorf("MAP 'app' key does not exist")
	}

	mapType, ok := bmapData.MAP[0]["type"].(string)
	if !ok {
		return fmt.Errorf("MAP 'type' key does not exist")
	}
	collectionName := mapType

	filter := bson.M{"_id": bmapData.Tx.Tx.H}

	bsonData := bson.M{
		"_id": filter["_id"],
		"tx":  bmapData.Tx,
		"blk": bmapData.Blk,
		"MAP": bmapData.MAP,
	}

	if bmapData.AIP != nil {
		bsonData["AIP"] = bmapData.AIP
	}

	// log.Println("Inserting into collection", collectionName)
	_, err = conn.UpsertOne(collectionName, filter, bsonData)

	return err
}
