package crawler

import (
	"bufio"
	"encoding/json"
	"log"
	"os"

	"github.com/bitcoinschema/go-bmap"
	"github.com/rohenaz/go-bmap-indexer/database"
	"go.mongodb.org/mongo-driver/bson"
)

// Worker for processing files
func Worker(readyFiles chan string) {
	for filename := range readyFiles {
		// Process the file
		ingest(filename)

		// After successful import, delete the file
		err := os.Remove(filename)
		if err != nil {
			log.Printf("Error deleting file %s: %v", filename, err)
		}
	}
}

// ingest JSONLD file and ingest each line as a mongo document
func ingest(filepath string) {
	conn := database.GetConnection()

	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filepath, err)
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
			log.Printf("[ERROR]: %v", err)
			continue
		}

		// 3 - insert into mongo
		if len(bmapData.MAP[0]) == 0 {
			log.Println("No MAP data.")
			continue
		}
		_, ok := bmapData.MAP[0]["app"].(string)
		if !ok {
			log.Println("Error: MAP 'app' key does not exist.")
			return
		}

		mapType, ok := bmapData.MAP[0]["type"].(string)
		if !ok {
			log.Println("Error: MAP 'type' key does not exist.")
			return
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

		log.Println("Inserting into collection", collectionName)
		_, err = conn.UpsertOne(collectionName, filter, bsonData)

		if err != nil {
			log.Printf("[ERROR]: %v", err)
			continue
		}
	}

	// Check for errors in the scanner
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v", filepath, err)
	}
}
