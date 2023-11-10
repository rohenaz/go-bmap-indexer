package p2p

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libsv/go-bk/wif"
	ma "github.com/multiformats/go-multiaddr"
)

var Started = false

// Node represents a node in the P2P network
type Node struct {
	Host   host.Host
	DHT    *dht.IpfsDHT
	PubSub *pubsub.PubSub
	Ctx    context.Context
}

// Start initializes the P2P node and connects it to the network
func Start() {

	// make this environment specific
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file loaded")
		if os.Getenv("ENVIRONMENT") == "development" {
			log.Fatal(err)
		}
	}

	privKey, err := getPrivateKeyFromEnv("BMAP_P2P_PK")
	if err != nil {
		log.Fatalf("Error getting private key: %s", err)
		return
	}

	// how do i use "https://go-bmap-indexer-production.up.railway.app/" ?
	var port = 11169
	listen, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))

	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/udp/0/quic", "/ip4/0.0.0.0/tcp/0", "/ip6/::/quic", "/ip6/::/tcp/0"),
		libp2p.DefaultTransports,
		libp2p.ListenAddrs(listen),
	)
	if err != nil {
		log.Fatalf("Error creating libp2p host: %s", err)
	}

	// The DHT and PubSub will use the background context since they manage their own lifecycle.
	d, err := dht.New(context.Background(), h)
	if err != nil {
		log.Fatalf("Error setting up DHT: %s", err)
	}

	ps, err := pubsub.NewGossipSub(context.Background(), h)
	if err != nil {
		log.Fatalf("Error setting up PubSub: %s", err)
	}

	node := &Node{
		Host:   h,
		DHT:    d,
		PubSub: ps,
		Ctx:    context.Background(),
	}

	Started = true

	// Implement your custom protocol handlers and pubsub subscription handlers here.

	log.Println("Node started. Listening on addresses", node.Host.Addrs())
}

// getPrivateKeyFromEnv loads the WIF-encoded private key from the environment variable and converts it to a libp2p private key
func getPrivateKeyFromEnv(envVar string) (crypto.PrivKey, error) {
	wifStr := os.Getenv(envVar)
	if wifStr == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envVar)
	}

	decodedWIF, err := wif.DecodeWIF(wifStr)
	if err != nil {
		return nil, fmt.Errorf("error decoding WIF: %s", err)
	}

	privKey, err := crypto.UnmarshalSecp256k1PrivateKey(decodedWIF.PrivKey.Serialise())
	if err != nil {
		return nil, fmt.Errorf("rror unmarshaling private key: %s", err)
	}

	return privKey, nil
}
