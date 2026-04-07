package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	mspID         = os.Getenv("MSP_ID")
	peerEndpoint  = os.Getenv("PEER_ENDPOINT")
	gatewayPeer   = os.Getenv("GATEWAY_PEER")
	channelName   = os.Getenv("CHANNEL_NAME")
	chaincodeName = os.Getenv("CHAINCODE_NAME")
	nodeName      = os.Getenv("NODE_NAME")
	role          = os.Getenv("ROLE")

	cryptoPath  = "/fabric-config/organizations/peerOrganizations/org2.example.com"
	certPath    = cryptoPath + "/users/User1@org2.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org2.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org2.example.com/tls/ca.crt"
)

// DetectionEvent from blockchain
type DetectionEvent struct {
	EventType   string    `json:"eventType"`
	IPAddress   string    `json:"ipAddress"`
	MACAddress  string    `json:"macAddress"`
	PreviousMAC string    `json:"previousMAC,omitempty"`
	Hostname    string    `json:"hostname"`
	RecordedBy  string    `json:"recordedBy"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
}

func main() {
	log.Println("============================================================")
	log.Printf("  🖥️  LAN NODE - %s (%s)", nodeName, role)
	log.Println("============================================================")
	log.Printf("Peer: %s (%s)\n", gatewayPeer, peerEndpoint)
	log.Printf("Channel: %s, Chaincode: %s\n", channelName, chaincodeName)
	log.Println()

	// If attacker role, just sleep (manual control)
	if role == "attacker" {
		log.Println("⚠️  ATTACKER MODE - Idle. Use docker exec to run attacks.")
		log.Println("   Example: docker exec lan-attacker /app/spoof-attack.sh 10.5.0.10 aa:bb:cc:dd:ee:99")
		select {} // Sleep forever
	}

	// Wait for Fabric network
	log.Println("⏳ Waiting 15s for Fabric network and router...")
	time.Sleep(15 * time.Second)

	// Connect to Fabric
	network := connectToFabric()
	defer network.Disconnect()

	log.Println("✅ Connected to blockchain")
	log.Println()

	// Get initial ARP table from blockchain
	log.Println("📥 Fetching initial ARP table from blockchain...")
	populateInitialARPCache(network.contract)

	// Start background traffic generation
	go generateTraffic()

	// Subscribe to ARP events
	log.Println("🎧 Subscribing to ARP events...")
	log.Println()
	subscribeToEvents(network)
}

func connectToFabric() *FabricNetwork {
	// Create gRPC connection
	clientConnection := newGrpcConnection()

	// Create identity
	id := newIdentity()
	sign := newSign()

	// Connect to gateway
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	return &FabricNetwork{
		gateway:  gw,
		contract: contract,
		conn:     clientConnection,
	}
}

type FabricNetwork struct {
	gateway  *client.Gateway
	contract *client.Contract
	conn     *grpc.ClientConn
}

func (fn *FabricNetwork) Disconnect() {
	fn.gateway.Close()
	fn.conn.Close()
}

func populateInitialARPCache(contract *client.Contract) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := contract.EvaluateTransaction(ctx, "GetAllARPEntries")
	if err != nil {
		log.Printf("⚠️  Failed to get ARP entries: %v", err)
		return
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(result, &entries); err != nil {
		log.Printf("⚠️  Failed to parse ARP entries: %v", err)
		return
	}

	log.Printf("📋 Found %d existing ARP entries\n", len(entries))

	for _, entry := range entries {
		ip := entry["ipAddress"].(string)
		mac := entry["macAddress"].(string)

		// Skip our own IP
		if strings.HasPrefix(ip, "10.5.0.") && !isLocalIP(ip) {
			addARPEntry(ip, mac)
			log.Printf("   ✅ Added: %s -> %s", ip, mac)
		}
	}

	log.Println()
}

func subscribeToEvents(network *FabricNetwork) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := network.gateway.GetNetwork(channelName).ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		log.Fatalf("Failed to subscribe to events: %v", err)
	}

	log.Println("📡 Listening for ARP events from blockchain...")
	log.Println()

	for event := range events {
		if event.EventName == "ARPDetectionEvent" {
			var detectionEvent DetectionEvent
			err := json.Unmarshal(event.Payload, &detectionEvent)
			if err != nil {
				log.Printf("⚠️  Failed to parse event: %v", err)
				continue
			}

			handleARPEvent(detectionEvent)
		}
	}
}

func handleARPEvent(event DetectionEvent) {
	timestamp := event.Timestamp.Format("15:04:05")

	switch event.EventType {
	case "new":
		log.Printf("[%s] 🆕 NEW DEVICE: %s -> %s", timestamp, event.IPAddress, event.MACAddress)
		addARPEntry(event.IPAddress, event.MACAddress)

	case "spoofing":
		log.Printf("[%s] 🚨 SPOOFING DETECTED!", timestamp)
		log.Printf("         IP: %s", event.IPAddress)
		log.Printf("         Old MAC: %s", event.PreviousMAC)
		log.Printf("         New MAC: %s", event.MACAddress)
		log.Printf("         ⛔ REJECTING malicious entry!")
		// Do NOT add to ARP cache - keep the legitimate one

	case "match":
		log.Printf("[%s] ✅ Valid update: %s -> %s", timestamp, event.IPAddress, event.MACAddress)
		// Update ARP cache (refresh)
		addARPEntry(event.IPAddress, event.MACAddress)

	default:
		log.Printf("[%s] ❓ Unknown event: %s", timestamp, event.EventType)
	}

	log.Println()
}

func addARPEntry(ip, mac string) {
	// Skip local IPs
	if isLocalIP(ip) {
		return
	}

	// Add static ARP entry
	cmd := exec.Command("ip", "neigh", "replace", ip, "lladdr", mac, "dev", "eth0", "nud", "permanent")
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️  Failed to add ARP entry: %v", err)
	}
}

func isLocalIP(ip string) bool {
	// Get local IP
	cmd := exec.Command("hostname", "-i")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	localIP := strings.TrimSpace(string(output))
	return ip == localIP
}

func generateTraffic() {
	// Wait before starting
	time.Sleep(20 * time.Second)

	log.Println("🔄 Starting background traffic generation...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Ping router to generate ARP traffic
		cmd := exec.Command("ping", "-c", "1", "-W", "1", "10.5.0.1")
		cmd.Run() // Ignore errors

		// Ping other nodes
		for i := 10; i <= 12; i++ {
			ip := fmt.Sprintf("10.5.0.%d", i)
			if !isLocalIP(ip) {
				cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
				cmd.Run()
			}
		}
	}
}

func newGrpcConnection() *grpc.ClientConn {
	cert, err := loadCertificate(tlsCertPath)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		log.Fatalf("Failed to create gRPC connection: %v", err)
	}

	return connection
}

func newIdentity() *identity.X509Identity {
	cert, err := loadCertificate(certPath)
	if err != nil {
		log.Fatalf("Failed to load certificate: %v", err)
	}

	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		log.Fatalf("Failed to create identity: %v", err)
	}

	return id
}

func newSign() identity.Sign {
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		log.Fatalf("Failed to read private key directory: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("No private key found in %s", keyPath)
	}

	privateKeyPEM, err := ioutil.ReadFile(filepath.Join(keyPath, files[0].Name()))
	if err != nil {
		log.Fatalf("Failed to read private key file: %v", err)
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
	}

	return sign
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}
