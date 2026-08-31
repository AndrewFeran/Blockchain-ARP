package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	flaskURL      = os.Getenv("FLASK_URL")

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

type arpObservation struct {
	srcIP  string
	srcMAC string
	dstIP  string
	opStr  string
}

type dashboardEvent struct {
	EventType   string `json:"eventType"`
	IPAddress   string `json:"ipAddress"`
	MACAddress  string `json:"macAddress"`
	PreviousMAC string `json:"previousMAC,omitempty"`
	Hostname    string `json:"hostname"`
	RecordedBy  string `json:"recordedBy"`
	Timestamp   string `json:"timestamp"`
	Message     string `json:"message"`
	TrialID     string `json:"trialId,omitempty"`
}

type arpEntry struct {
	MACAddress string `json:"macAddress"`
	IsExpired  bool   `json:"isExpired"`
}

func main() {
	log.Println("============================================================")
	log.Println("  BLOCKCHAIN ROUTER - ARP Authority")
	log.Println("============================================================")
	log.Printf("Node Name: %s\n", nodeName)
	log.Printf("Peer: %s (%s)\n", gatewayPeer, peerEndpoint)
	log.Printf("Channel: %s, Chaincode: %s\n", channelName, chaincodeName)
	log.Println()

	log.Println("Waiting 10s for Fabric network...")
	time.Sleep(10 * time.Second)

	contract := connectToFabric()
	defer contract.Disconnect()

	log.Println("Connected to blockchain")
	log.Println()

	log.Println("Starting ARP packet capture on eth0...")
	captureARP(contract)
}

func connectToFabric() *FabricContract {
	clientConnection := newGrpcConnection()
	id := newIdentity()
	sign := newSign()

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
	handle, err := pcap.OpenLive("eth0", 1600, true, pcap.BlockForever)
	if err != nil {
		log.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()

	err = handle.SetBPFFilter("arp")
	if err != nil {
		log.Fatalf("Failed to set BPF filter: %v", err)
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	log.Println("Listening for ARP packets...")
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
	observation := parseARPObservation(arp)
	log.Printf("ARP %s: %s (%s) -> %s", observation.opStr, observation.srcIP, observation.srcMAC, observation.dstIP)

	trialID := newTrialID()
	logBenchmarkEvent("router_captured", trialID, observation.srcIP, observation.srcMAC)

	event := classifyDashboardEvent(contract, observation.srcIP, observation.srcMAC, trialID)
	err := writeToBlockchain(contract, observation.srcIP, observation.srcMAC, trialID)
	if err != nil {
		log.Printf("Failed to write to blockchain: %v", err)
	} else {
		logBenchmarkEvent("fabric_submitted", trialID, observation.srcIP, observation.srcMAC)
		log.Printf("Recorded to blockchain: %s -> %s", observation.srcIP, observation.srcMAC)
		postDashboardEvent(event)
	}
	log.Println()
}

func parseARPObservation(arp *layers.ARP) arpObservation {
	opStr := "Unknown"
	if arp.Operation == 1 {
		opStr = "Request"
	} else if arp.Operation == 2 {
		opStr = "Reply"
	}

	return arpObservation{
		srcIP:  fmt.Sprintf("%d.%d.%d.%d", arp.SourceProtAddress[0], arp.SourceProtAddress[1], arp.SourceProtAddress[2], arp.SourceProtAddress[3]),
		srcMAC: fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", arp.SourceHwAddress[0], arp.SourceHwAddress[1], arp.SourceHwAddress[2], arp.SourceHwAddress[3], arp.SourceHwAddress[4], arp.SourceHwAddress[5]),
		dstIP:  fmt.Sprintf("%d.%d.%d.%d", arp.DstProtAddress[0], arp.DstProtAddress[1], arp.DstProtAddress[2], arp.DstProtAddress[3]),
		opStr:  opStr,
	}
}

// ---------------------------------------------------------------------------
// Section IV stub observations — mirrors DHCPObservation / SwitchObservation
// in the chaincode. Private keys match the public keys embedded there.
// ---------------------------------------------------------------------------

const stubDHCPPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgj7Es4NtY7dWBCpXu
6NawD+9SpBFvO85UerOqIqPgFL+hRANCAAQuMsYkDBRTQGue0Fu//yxziDOpJf5d
9GO4gyKpUDKqg7bpZKxZdH2cqjNFRV3xQmCZCSKB5vdqtB0MiGNkxf1P
-----END PRIVATE KEY-----`

const stubSwitchPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgJQ3c8Kpj9Mrf27Va
fgmbAW1qcAksDAkAZ11P5p9NuhyhRANCAAS77LosTfIucmyZMLOiOLt7Ia0X94pO
pFGSUCocg0BabxVkGE/dlmMfezE7inlmtumlNZF7BGm3OkDZm4bIHs1e
-----END PRIVATE KEY-----`

type stubDHCPObservation struct {
	IPAddress   string `json:"ipAddress"`
	MACAddress  string `json:"macAddress"`
	Subnet      string `json:"subnet"`
	LeaseExpiry int64  `json:"leaseExpiry"`
	Signature   string `json:"signature"`
}

type stubSwitchObservation struct {
	MACAddress string `json:"macAddress"`
	Port       string `json:"port"`
	VLAN       string `json:"vlan"`
	Timestamp  int64  `json:"timestamp"`
	Signature  string `json:"signature"`
}

// signObsContent signs content with the PKCS8 ECDSA private key and returns
// a base64-encoded ASN.1 DER (r, s) signature.
func signObsContent(privKeyPEM, content string) string {
	block, _ := pem.Decode([]byte(privKeyPEM))
	if block == nil {
		log.Fatalf("failed to decode private key PEM")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}
	ecPriv := priv.(*ecdsa.PrivateKey)
	hash := sha256.Sum256([]byte(content))
	r, s, err := ecdsa.Sign(cryptorand.Reader, ecPriv, hash[:])
	if err != nil {
		log.Fatalf("failed to sign observation: %v", err)
	}
	derBytes, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		log.Fatalf("failed to marshal signature: %v", err)
	}
	return base64.StdEncoding.EncodeToString(derBytes)
}

func makeStubDHCPObsJSON(ip, mac string) string {
	obs := stubDHCPObservation{
		IPAddress:   ip,
		MACAddress:  mac,
		Subnet:      "10.5.0.0/24",
		LeaseExpiry: time.Now().Add(24 * time.Hour).Unix(),
	}
	content := "dhcp|" + obs.IPAddress + "|" + obs.MACAddress + "|" + obs.Subnet + "|" + strconv.FormatInt(obs.LeaseExpiry, 10)
	obs.Signature = signObsContent(stubDHCPPrivateKeyPEM, content)
	b, err := json.Marshal(obs)
	if err != nil {
		log.Fatalf("failed to marshal DHCP observation: %v", err)
	}
	return string(b)
}

func makeStubSwitchObsJSON(mac string) string {
	obs := stubSwitchObservation{
		MACAddress: mac,
		Port:       "GigabitEthernet0/1",
		VLAN:       "100",
		Timestamp:  time.Now().Unix(),
	}
	content := "switch|" + obs.MACAddress + "|" + obs.Port + "|" + obs.VLAN + "|" + strconv.FormatInt(obs.Timestamp, 10)
	obs.Signature = signObsContent(stubSwitchPrivateKeyPEM, content)
	b, err := json.Marshal(obs)
	if err != nil {
		log.Fatalf("failed to marshal switch observation: %v", err)
	}
	return string(b)
}

func writeToBlockchain(contract *FabricContract, ipAddress, macAddress, trialID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dhcpObsJSON := makeStubDHCPObsJSON(ipAddress, macAddress)
	switchObsJSON := makeStubSwitchObsJSON(macAddress)

	_, err := contract.contract.SubmitWithContext(
		ctx,
		"RegisterBinding",
		client.WithArguments(dhcpObsJSON, switchObsJSON),
		client.WithTransient(map[string][]byte{"trialID": []byte(trialID)}),
	)
	return err
}

func classifyDashboardEvent(contract *FabricContract, ipAddress, macAddress, trialID string) dashboardEvent {
	event := dashboardEvent{
		EventType:  "new",
		IPAddress:  ipAddress,
		MACAddress: macAddress,
		Hostname:   nodeName,
		RecordedBy: "router",
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Message:    fmt.Sprintf("New device: %s -> %s", ipAddress, macAddress),
		TrialID:    trialID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := contract.contract.EvaluateWithContext(ctx, "GetCurrentARPEntry", client.WithArguments(ipAddress))
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			log.Printf("Dashboard event classification fell back to new: %v", err)
		}
		return event
	}

	var existing arpEntry
	if err := json.Unmarshal(result, &existing); err != nil {
		log.Printf("Dashboard event classification fell back to new: %v", err)
		return event
	}

	if existing.IsExpired {
		event.Message = fmt.Sprintf("Re-registered expired device: %s -> %s", ipAddress, macAddress)
		return event
	}

	if existing.MACAddress != macAddress {
		event.EventType = "spoofing"
		event.PreviousMAC = existing.MACAddress
		event.Message = fmt.Sprintf("MAC CHANGED! %s: %s -> %s", ipAddress, existing.MACAddress, macAddress)
		return event
	}

	event.EventType = "match"
	event.Message = fmt.Sprintf("Valid update: %s -> %s", ipAddress, macAddress)
	return event
}

func postDashboardEvent(event dashboardEvent) {
	if flaskURL == "" {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal dashboard event: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, flaskURL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("Failed to build dashboard request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to post dashboard event: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Dashboard event post returned status %d", resp.StatusCode)
	}
}

func newTrialID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func logBenchmarkEvent(stage, trialID, ipAddress, macAddress string) {
	log.Printf("BENCHMARK_EVENT stage=%s trial=%s ip=%s mac=%s ts=%d",
		stage, trialID, ipAddress, macAddress, time.Now().UnixMilli())
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
