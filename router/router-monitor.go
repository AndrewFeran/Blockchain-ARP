package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
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

	cryptoPath  = "/fabric-config/organizations/peerOrganizations/org1.example.com"
	certPath    = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
)

type ARPPacket struct {
	SrcIP  string
	SrcMAC string
	DstIP  string
	DstMAC string
	OpCode uint16
}

func main() {
	log.Println("============================================================")
	log.Println("  🛡️  BLOCKCHAIN ROUTER - ARP Authority")
	log.Println("============================================================")
	log.Printf("Node Name: %s\n", nodeName)
	log.Printf("Peer: %s (%s)\n", gatewayPeer, peerEndpoint)
	log.Printf("Channel: %s, Chaincode: %s\n", channelName, chaincodeName)
	log.Println()

	// Wait for Fabric network to be ready
	log.Println("⏳ Waiting 10s for Fabric network...")
	time.Sleep(10 * time.Second)

	// Connect to Fabric
	contract := connectToFabric()
	defer contract.Disconnect()

	log.Println("✅ Connected to blockchain")
	log.Println()

	// Start ARP capture
	log.Println("🎧 Starting ARP packet capture on eth0...")
	captureARP(contract)
}

func connectToFabric() *FabricContract {
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

	return &FabricContract{
		gateway:  gw,
		contract: contract,
		conn:     clientConnection,
	}
}

type FabricContract struct {
	gateway  *client.Gateway
	contract *client.Contract
	conn     *grpc.ClientConn
}

func (fc *FabricContract) Disconnect() {
	fc.gateway.Close()
	fc.conn.Close()
}

func captureARP(contract *FabricContract) {
	// Open network interface
	handle, err := pcap.OpenLive("eth0", 1600, true, pcap.BlockForever)
	if err != nil {
		log.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()

	// Set filter for ARP packets only
	err = handle.SetBPFFilter("arp")
	if err != nil {
		log.Fatalf("Failed to set BPF filter: %v", err)
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	log.Println("📡 Listening for ARP packets...")
	log.Println()

	for packet := range packetSource.Packets() {
		arpLayer := packet.Layer(layers.LayerTypeARP)
		if arpLayer != nil {
			arp := arpLayer.(*layers.ARP)
			handleARPPacket(arp, contract)
		}
	}
}

func handleARPPacket(arp *layers.ARP, contract *FabricContract) {
	srcIP := fmt.Sprintf("%d.%d.%d.%d", arp.SourceProtAddress[0], arp.SourceProtAddress[1], arp.SourceProtAddress[2], arp.SourceProtAddress[3])
	srcMAC := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", arp.SourceHwAddress[0], arp.SourceHwAddress[1], arp.SourceHwAddress[2], arp.SourceHwAddress[3], arp.SourceHwAddress[4], arp.SourceHwAddress[5])

	dstIP := fmt.Sprintf("%d.%d.%d.%d", arp.DstProtAddress[0], arp.DstProtAddress[1], arp.DstProtAddress[2], arp.DstProtAddress[3])

	opCode := arp.Operation
	opStr := "Unknown"
	if opCode == 1 {
		opStr = "Request"
	} else if opCode == 2 {
		opStr = "Reply"
	}

	log.Printf("📨 ARP %s: %s (%s) -> %s", opStr, srcIP, srcMAC, dstIP)

	// Write to blockchain
	err := writeToBlockchain(contract, srcIP, srcMAC)
	if err != nil {
		log.Printf("⚠️  Failed to write to blockchain: %v", err)
	} else {
		log.Printf("✅ Recorded to blockchain: %s -> %s", srcIP, srcMAC)
	}
	log.Println()
}

func writeToBlockchain(contract *FabricContract, ipAddress, macAddress string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := contract.contract.SubmitTransaction(
		ctx,
		"RecordARPEntry",
		ipAddress,
		macAddress,
		"eth0",
		nodeName,
		"dynamic",
		"reachable",
		"router",
	)

	return err
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
