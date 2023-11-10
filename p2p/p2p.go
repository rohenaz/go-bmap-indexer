package p2p

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
		if os.Getenv("ENVIROMENT") == "development" {
			log.Fatal(err)
		}

	}

	privKey, err := getPrivateKeyFromEnv("BMAP_P2P_PK")
	if err != nil {
		log.Fatalf("Error getting private key: %s", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		log.Fatalf("Error creating libp2p host: %s", err)
	}

	// Attempt to connect to the bootstrap node
	bootstrapPeers, err := resolveBootstrapPeers("go-bmap-indexer-production.up.railway.app", 11169)
	if err != nil {
		log.Fatalf("Error resolving bootstrap peers: %s", err)
	}

	for _, peerAddr := range bootstrapPeers {
		log.Println("Attempting to connect to bootstrap peer:", peerAddr)
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			log.Println("Error creating AddrInfo:", err)
			continue
		}
		if err := h.Connect(context.Background(), *peerInfo); err != nil {
			log.Println("Error connecting to bootstrap peer:", err)
		}
	}

	Started = true

	log.Println("Node started with ID:", h.ID(), "and addresses: ", h.Addrs())
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
		return nil, fmt.Errorf("error unmarshaling private key: %s", err)
	}

	return privKey, nil
}

// resolveBootstrapPeers resolves the DNS and returns a slice of multiaddresses for the bootstrap nodes
func resolveBootstrapPeers(url string, port int) ([]ma.Multiaddr, error) {
	ips, err := net.LookupIP(url)
	if err != nil {
		return nil, err
	}

	var peers []ma.Multiaddr
	for _, ip := range ips {
		addrStr := fmt.Sprintf("/ip4/%s/tcp/%d", ip.String(), port)
		ma, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			log.Println("Error creating multiaddress for IP:", ip.String(), err)
			continue
		}
		peers = append(peers, ma)
	}
	return peers, nil
}
