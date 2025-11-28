package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	mspID        = "Org1MSP"
	cryptoPath   = "../fabric-samples/test-network/organizations/peerOrganizations/org1.example.com"
	certPath     = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/cert.pem"
	keyPath      = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath  = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
	peerEndpoint = "localhost:7051"
	gatewayPeer  = "peer0.org1.example.com"

	channelName   = "mychannel"
	chaincodeName = "arptracker"
	flaskURL      = "http://localhost:5000/api/event"
)

// DetectionEvent represents an ARP event from the blockchain
type DetectionEvent struct {
	EventType   string    `json:"eventType"`
	IPAddress   string    `json:"ipAddress"`
	MACAddress  string    `json:"macAddress"`
	PreviousMAC string    `json:"previousMAC,omitempty"`
	Hostname    string    `json:"hostname"`
	Interface   string    `json:"interface"`
	RecordedBy  string    `json:"recordedBy"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
}

func main() {
	log.Println("============================================================")
	log.Println("  ARP Tracker - Real-time Event Listener (Go)")
	log.Println("============================================================")
	log.Println()

	// Check if Flask is running
	checkFlask()

	// Create gRPC connection
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	// Create identity
	id := newIdentity()
	sign := newSign()

	// Create gateway connection
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
	defer gw.Close()

	network := gw.GetNetwork(channelName)

	log.Printf("🎧 Listening for chaincode events on channel '%s'...", channelName)
	log.Printf("📤 Forwarding events to Flask at %s", flaskURL)
	log.Println()

	// Subscribe to chaincode events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := network.ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		log.Fatalf("Failed to subscribe to chaincode events: %v", err)
	}

	// Listen for events
	for event := range events {
		log.Printf("📨 Received event: %s", event.EventName)

		if event.EventName == "ARPDetectionEvent" {
			var detectionEvent DetectionEvent
			err := json.Unmarshal(event.Payload, &detectionEvent)
			if err != nil {
				log.Printf("⚠️  Failed to parse event: %v", err)
				continue
			}

			// Forward to Flask
			forwardToFlask(detectionEvent)
		}
	}
}

func checkFlask() {
	resp, err := http.Get("http://localhost:5000")
	if err != nil {
		log.Println("⚠️  WARNING: Flask dashboard may not be running at localhost:5000")
		log.Println("   Start it with: cd dashboard && python3 app.py")
	} else {
		resp.Body.Close()
		log.Println("✅ Flask dashboard is running")
	}
	log.Println()
}

func forwardToFlask(event DetectionEvent) {
	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("⚠️  Failed to marshal event: %v", err)
		return
	}

	resp, err := http.Post(flaskURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("⚠️  Failed to forward to Flask: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		switch event.EventType {
		case "spoofing":
			log.Printf("🚨 SPOOFING DETECTED! IP: %s, Old: %s, New: %s",
				event.IPAddress, event.PreviousMAC, event.MACAddress)
		case "new":
			log.Printf("🆕 New device: IP: %s, MAC: %s",
				event.IPAddress, event.MACAddress)
		default:
			log.Printf("✅ Valid update: IP: %s, MAC: %s",
				event.IPAddress, event.MACAddress)
		}
	} else {
		log.Printf("⚠️  Flask returned status %d", resp.StatusCode)
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