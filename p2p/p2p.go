package p2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bitcoinschema/go-bmap"
	"github.com/fxamacker/cbor"
	"github.com/ipfs/go-cid"
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
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	"github.com/rohenaz/go-bmap-indexer/cache"
	"github.com/rohenaz/go-bmap-indexer/persist"
	"github.com/ttacon/chalk"
)

var Started = false
var ReadyBlock uint32
var kademliaDHT *dht.IpfsDHT
var namespace = "bmap"

// Node represents a node in the P2P network
type Node struct {
	Host   host.Host
	DHT    *dht.IpfsDHT
	PubSub *pubsub.PubSub
	Ctx    context.Context
}

type LineData struct {
	Line   []byte
	Height string
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
			dutil.Advertise(ctx, routingDiscovery, namespace)
			log.Println("Successfully announced!")

			// Now, look for others who have announced
			// This is like your friend telling you the location to meet you.
			log.Println("Searching for other peers...")
			peerChan, err := routingDiscovery.FindPeers(ctx, namespace)
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

			// announce what we have

			//			log.Println("Announcing CID:", cid.String())
			// create a cid from these bytes
			//		kademliaDHT.Provide(context.Background(), *cid, true)

			select {}
		}
	} else {
		log.Println("No bootstrap peer ID provided")
	}

	log.Println("Node started with ID:", h.ID(), "and addresses: ", h.Addrs())

	// empty channel
	<-make(chan struct{})
}

// CreateFiles will import the jsonld files in the data folder and create individual cbor encoded files for every line (parsed bmap tx)
func CreateFiles() {
	Started = true
	cache.Connect()
	fmt.Println("Preparing data...")

	// Get files from ./data directory
	files, err := os.ReadDir("./data")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			log.Println("Importing file:", file.Name())
			height := strings.Split(file.Name(), ".")[0]
			importFile("./data/"+file.Name(), height)
		}
	}
}

func importFile(file string, height string) {
	// open the file
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// create a scanner
	scanner := bufio.NewScanner(f)

	// create a channel
	ch := make(chan LineData, 1000)

	// create a waitgroup
	var wg sync.WaitGroup

	// start the workers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go worker(ch, &wg, height)
	}

	// read the file
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))

		// prevent race conditions by copying
		copy(line, scanner.Bytes())

		ch <- LineData{Line: line, Height: height}
	}

	// close the channel
	close(ch)

	// wait for the workers to finish
	wg.Wait()
}

func worker(ch chan LineData, wg *sync.WaitGroup, height string) {
	defer wg.Done()
	for lineData := range ch {
		// Process the line with the block height
		txid, cid, err := processLine(lineData.Line, lineData.Height)
		if err != nil {
			log.Println(err)
		}

		if txid == nil {
			log.Println("txid is nil")
			continue
		}

		if cid == nil {
			log.Println("cid is nil")
			continue
		}
	}

	// delete the json file ONLY IF it is not the "highest" file
	// convert height to uint32

	heightNum, err := strconv.ParseUint(height, 10, 32)
	if err != nil {
		log.Fatal(err)
	}

	if uint32(heightNum) <= ReadyBlock && heightNum != 0 {

		// TODO: We need to handle 0 case which is mempool
		fmt.Printf("%sDeleting file in p2p worker %s%s\n", chalk.Cyan, height+".json", chalk.Reset)

		err := os.Remove("./data/" + height + ".json")
		if err != nil {
			fmt.Printf("%s%s %s: %v%s\n", chalk.Cyan, "Error deleting file", height+".json", err, chalk.Reset)
		}
	}

}

func processLine(line []byte, height string) (txid *string, cid *cid.Cid, err error) {
	var tx bmap.Tx
	// Assuming line is a JSON object, unmarshal it into a map
	if err := json.Unmarshal(line, &tx); err != nil {
		log.Println("Error unmarshaling line:", err)
		return nil, nil, err
	}

	// Encode the map to CBOR
	cborData, err := cbor.Marshal(tx, cbor.EncOptions{})
	if err != nil {
		log.Println("Error encoding to CBOR:", err)
		return nil, nil, err
	}

	// Write cborData to a file in data/<height>/<txid>.cbor
	// makea copy
	txh := tx.Tx.Tx.H
	txid = &txh

	filePath := fmt.Sprintf("./data/%s/%s.cbor", height, *txid)
	persist.SaveCBOR(filePath, cborData)

	log.Printf("Saved CBOR file: %s", filePath)

	cid, err = GenerateCID(cborData)
	if err != nil {
		log.Println("Error generating CID:", err)
		return nil, nil, err
	}

	// announce availability on the DHT
	// https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-rendezvous/chat.go
	err = cache.Set("p2p-bmap-"+height+"-"+*txid, cid.String())
	if err != nil {
		return nil, nil, err
	}

	return txid, cid, nil
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

func GenerateCID(content []byte) (contentID *cid.Cid, err error) {

	// Create a cid manually by specifying the 'prefix' parameters
	pref := cid.Prefix{
		Version:  1,
		Codec:    uint64(mc.Raw),
		MhType:   mh.DBL_SHA2_256,
		MhLength: -1, // default length
	}

	// And then feed it some data
	c, err := pref.Sum(content)
	if err != nil {
		return nil, err
	}

	fmt.Println("Created CID: ", c)

	return &c, nil
}
