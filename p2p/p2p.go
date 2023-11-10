package p2p

import (
	"bufio"
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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
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

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

// Start initializes the P2P node and connects it to the network
func Start() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file loaded")
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
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/11169", "/ip6/::/tcp/11169"),
	)
	if err != nil {
		log.Fatalf("Error creating libp2p host: %s", err)
	}

	// define protocol
	h.SetStreamHandler("/bmap/1.0.0", handleStream)

	bootstrapPeerID := os.Getenv("BOOTSTRAP_PEER_ID")
	if bootstrapPeerID != "" {
		// Attempt to connect to the bootstrap node
		bootstrapPeers, err := resolveBootstrapPeers("viaduct.proxy.rlwy.net", 49648, bootstrapPeerID)
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

			// I think this adds the peer to the peerstore?
			h.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

			log.Println("Connected to bootstrap peer:", peerInfo.ID)
			// now that we're connected, we can open a stream to this peer
			stream, err := h.NewStream(context.Background(), peerInfo.ID, "/bmap/1.0.0")
			if err != nil {
				log.Println("Error opening stream to bootstrap peer:", err)
				continue
			}
			log.Println("Opened stream to bootstrap peer:", stream.Conn().RemotePeer())

			// Start a DHT, for use in peer discovery. We can't just make a new DHT
			// client because we want each peer to maintain its own local copy of the
			// DHT, so that the bootstrapping node of the DHT can go down without
			// inhibiting future peer discovery.
			ctx := context.Background()
			kademliaDHT, err := dht.New(ctx, h)
			if err != nil {
				panic(err)
			}

			routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
			dutil.Advertise(ctx, routingDiscovery, "Hola world!")
			log.Println("Successfully announced!")

			// Now, look for others who have announced
			// This is like your friend telling you the location to meet you.
			log.Println("Searching for other peers...")
			peerChan, err := routingDiscovery.FindPeers(ctx, "Hola world!")
			if err != nil {
				panic(err)
			}

			for peer := range peerChan {
				if peer.ID == h.ID() {
					continue
				}
				log.Println("Found peer:", peer)

				log.Println("Connecting to:", peer)
				stream, err := h.NewStream(ctx, peer.ID, protocol.ID("/bmap/1.0.0"))

				if err != nil {
					log.Println("Connection failed:", err)
					continue
				} else {
					rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

					go writeData(rw)
					go readData(rw)
				}

				log.Println("Connected to:", peer)
			}

			select {}
		}
	} else {
		log.Println("No bootstrap peer ID provided")
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

func resolveBootstrapPeers(domain string, port int, peerID string) ([]ma.Multiaddr, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return nil, err
	}

	var peers []ma.Multiaddr
	for _, ip := range ips {
		var addrStr string
		if ip.To4() != nil {
			// IPv4 address
			addrStr = fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip.String(), port, peerID)
		} else {
			// IPv6 address
			addrStr = fmt.Sprintf("/ip6/%s/tcp/%d/p2p/%s", ip.String(), port, peerID)
		}
		ma, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			log.Println("Error creating multiaddress for IP:", ip.String(), err)
			continue
		}
		peers = append(peers, ma)
	}
	return peers, nil
}
