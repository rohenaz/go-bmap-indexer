package persist

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/fxamacker/cbor"
)

var MarshalCBOR = func(v interface{}) (io.Reader, error) {
	b, err := cbor.Marshal(v, cbor.EncOptions{})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// Marshal is a function that marshals the object into an
// io.Reader.
// By default, it uses the JSON marshaller.
var MarshalIndent = func(v interface{}) (io.Reader, error) {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// Marshal is a function that marshals the object into an
// io.Reader.
// This version produces compact, single-line JSON.
var Marshal = func(v interface{}) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// Unmarshal is a function that unmarshals the data from the
// reader into the specified value.
// By default, it uses the JSON unmarshaller.
var Unmarshal = func(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

var lock sync.Mutex

// Save saves a representation of v to the file at path.
func SaveLine(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open the file in append mode or create if it doesn't exist
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := Marshal(v)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	// Write a newline after each piece of data to make it NDJSON format
	_, err = f.WriteString("\n")
	return err
}

func SaveCBOR(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()

	// Construct the full path
	fullPath := filepath.Join(path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := MarshalCBOR(v)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return err
}

// Save a json representation of v to the file at path.
func Save(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := MarshalIndent(v)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return err
}

// Load loads the file at path into v.
// Use os.IsNotExist() to see if the returned error is due
// to the file being missing.
func Load(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Unmarshal(f, v)
}
