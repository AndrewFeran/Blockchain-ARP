# Hyperledger Fabric ARP Tracker

A blockchain-based system for tracking and auditing Address Resolution Protocol (ARP) entries using Hyperledger Fabric. This chaincode provides an immutable ledger for recording ARP table entries, detecting MAC address changes, and maintaining complete history for network security and compliance purposes.

## 🎯 Features

- **Record ARP Entries**: Store IP-to-MAC address mappings on the blockchain
- **Historical Tracking**: Maintain complete history of all ARP changes for each IP address
- **MAC Change Detection**: Detect when an IP address changes its associated MAC address (potential ARP spoofing)
- **Query Capabilities**: Search by IP address, MAC address, or retrieve all entries
- **Immutable Audit Trail**: Blockchain-backed records that cannot be tampered with
- **Multi-Organization Support**: Built on Hyperledger Fabric's permissioned blockchain model

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Chaincode Functions](#chaincode-functions)
- [Usage Examples](#usage-examples)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [Contributing](#contributing)

## 🔧 Prerequisites

Before you begin, ensure you have the following installed on your Ubuntu system:

- **Docker**: v20.10 or later
- **Docker Compose**: v2.0 or later
- **Go**: v1.23 or later
- **Git**: v2.0 or later
- **Hyperledger Fabric Samples**: v2.5 or later

### Verify Installations

```bash
docker --version
docker compose version
go version
git --version
```

## 📦 Installation

### 1. Install Hyperledger Fabric

```bash
# Create working directory
mkdir -p ~/fabric
cd ~/fabric

# Download Fabric samples, binaries, and Docker images
curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh
chmod +x install-fabric.sh
./install-fabric.sh docker binary samples
```

### 2. Clone/Create ARP Chaincode

The ARP chaincode is located in `~/fabric/arp-chaincode/`. The project structure is:

```
~/fabric/arp-chaincode/
├── arp-chaincode.go    # Main chaincode implementation
├── go.mod              # Go module definition
├── go.sum              # Go dependencies checksum
└── Dockerfile          # Container image for chaincode
```

### 3. Initialize Go Module (if starting fresh)

```bash
cd ~/fabric/arp-chaincode
go mod init arp-chaincode
go get github.com/hyperledger/fabric-contract-api-go
go mod tidy
```

## 🚀 Quick Start

### Complete Setup in One Go

```bash
# Navigate to test network
cd ~/fabric/fabric-samples/test-network

# Clean up any existing network
./network.sh down
docker rm -f peer0org1_arptracker_ccaas peer0org2_arptracker_ccaas

# Start the Fabric network and create channel
./network.sh up createChannel

# Set chaincode package ID
export PACKAGE_ID=arptracker_1.0:6fe197fc14b84792672b35bd4707f41154d4aad207b6bbc9f01269d46fdcc45f

# Build and start chaincode containers
cd ~/fabric/arp-chaincode
docker build -t arptracker_ccaas_image:latest .

docker run -d --name peer0org1_arptracker_ccaas \
  --network fabric_test \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -e CORE_CHAINCODE_ID_NAME=$PACKAGE_ID \
  arptracker_ccaas_image:latest

docker run -d --name peer0org2_arptracker_ccaas \
  --network fabric_test \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -e CORE_CHAINCODE_ID_NAME=$PACKAGE_ID \
  arptracker_ccaas_image:latest

# Set up peer CLI environment
cd ~/fabric/fabric-samples/test-network
export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/../config
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

# Test the installation
peer chaincode invoke -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel -n arptracker \
  --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{"function":"RecordARPEntry","Args":["192.168.1.100","AA:BB:CC:DD:EE:FF","eth0","server1","dynamic","reachable","gateway1"]}'

echo "✅ ARP Tracker is up and running!"
```

## 📁 Project Structure

```
hyperledger-fabric-arp-tracker/
├── README.md                           # This file
├── arp-chaincode/                      # Chaincode source code
│   ├── arp-chaincode.go               # Main chaincode implementation
│   ├── go.mod                         # Go module file
│   ├── go.sum                         # Go dependencies
│   └── Dockerfile                     # Chaincode container image
└── fabric-samples/                     # Hyperledger Fabric test network
    └── test-network/                   # Test network scripts
```

## 🔌 Chaincode Functions

### Data Structures

#### ARPEntry
```go
type ARPEntry struct {
    IPAddress   string `json:"ipAddress"`   // IP address
    MACAddress  string `json:"macAddress"`  // MAC address
    Interface   string `json:"interface"`   // Network interface
    Hostname    string `json:"hostname"`    // Hostname
    EntryType   string `json:"entryType"`   // "static" or "dynamic"
    State       string `json:"state"`       // "reachable", "stale", "delay", "probe", "failed"
    RecordedBy  string `json:"recordedBy"`  // System that recorded this entry
}
```

### Available Functions

#### 1. RecordARPEntry
Records a new ARP entry to the ledger.

**Parameters:**
- `ipAddress` (string): IP address
- `macAddress` (string): MAC address
- `interface` (string): Network interface (e.g., "eth0")
- `hostname` (string): Hostname
- `entryType` (string): "static" or "dynamic"
- `state` (string): ARP entry state
- `recordedBy` (string): Identifier of recording system

#### 2. GetCurrentARPEntry
Retrieves the current ARP entry for a given IP address.

**Parameters:**
- `ipAddress` (string): IP address to query

**Returns:** ARPEntry object

#### 3. GetARPHistory
Retrieves complete history of ARP entries for a given IP address.

**Parameters:**
- `ipAddress` (string): IP address to query

**Returns:** ARPHistory object containing all historical entries

#### 4. GetAllARPEntries
Retrieves all current ARP entries in the ledger.

**Parameters:** None

**Returns:** Array of ARPEntry objects

#### 5. QueryARPByMAC
Finds all IP addresses associated with a specific MAC address.

**Parameters:**
- `macAddress` (string): MAC address to search for

**Returns:** Array of ARPEntry objects

#### 6. DetectMACChange
Checks if a MAC address has changed for a given IP address.

**Parameters:**
- `ipAddress` (string): IP address to check
- `currentMAC` (string): Current MAC address to compare

**Returns:** MACChangeResult object

#### 7. DeleteARPEntry
Removes an ARP entry from the ledger (cleanup operation).

**Parameters:**
- `ipAddress` (string): IP address to delete

## 💡 Usage Examples

### Record an ARP Entry

```bash
peer chaincode invoke -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel -n arptracker \
  --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{"function":"RecordARPEntry","Args":["192.168.1.100","AA:BB:CC:DD:EE:FF","eth0","server1","dynamic","reachable","gateway1"]}'
```

### Query a Specific Entry

```bash
peer chaincode query -C mychannel -n arptracker \
  -c '{"function":"GetCurrentARPEntry","Args":["192.168.1.100"]}'
```

**Output:**
```json
{
  "ipAddress": "192.168.1.100",
  "macAddress": "AA:BB:CC:DD:EE:FF",
  "interface": "eth0",
  "hostname": "server1",
  "entryType": "dynamic",
  "state": "reachable",
  "recordedBy": "gateway1"
}
```

### Get All Entries

```bash
peer chaincode query -C mychannel -n arptracker \
  -c '{"function":"GetAllARPEntries","Args":[]}'
```

### Query by MAC Address

```bash
peer chaincode query -C mychannel -n arptracker \
  -c '{"function":"QueryARPByMAC","Args":["AA:BB:CC:DD:EE:FF"]}'
```

### Get Historical Changes

```bash
peer chaincode query -C mychannel -n arptracker \
  -c '{"function":"GetARPHistory","Args":["192.168.1.100"]}'
```

### Detect MAC Address Changes

```bash
peer chaincode query -C mychannel -n arptracker \
  -c '{"function":"DetectMACChange","Args":["192.168.1.100","11:22:33:44:55:66"]}'
```

### Delete an Entry

```bash
peer chaincode invoke -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
  -C mychannel -n arptracker \
  --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
  --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
  -c '{"function":"DeleteARPEntry","Args":["192.168.1.100"]}'
```

## 🛠️ Troubleshooting

### Common Issues

#### Issue: "connection refused" on localhost:7051

**Cause:** Not in the correct directory or environment variables not set.

**Fix:**
```bash
cd ~/fabric/fabric-samples/test-network
# Re-run environment variable setup (see Quick Start section)
```

#### Issue: "ProposalResponsePayloads do not match"

**Cause:** Chaincode contains non-deterministic code (e.g., different timestamps on different peers).

**Fix:** Ensure your chaincode doesn't use `time.Now()` or other non-deterministic functions. Use transaction timestamps instead.

#### Issue: Chaincode containers exit immediately

**Cause:** Missing environment variable or code error.

**Fix:**
```bash
# Check logs
docker logs peer0org1_arptracker_ccaas
docker logs peer0org2_arptracker_ccaas

# Ensure PACKAGE_ID is set
export PACKAGE_ID=arptracker_1.0:6fe197fc14b84792672b35bd4707f41154d4aad207b6bbc9f01269d46fdcc45f
```

#### Issue: Cannot find peer command

**Cause:** PATH not set correctly.

**Fix:**
```bash
cd ~/fabric/fabric-samples/test-network
export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/../config
```

### Diagnostic Commands

```bash
# Check network status
docker ps

# Check chaincode logs
docker logs peer0org1_arptracker_ccaas
docker logs peer0org2_arptracker_ccaas

# Check peer logs
docker logs peer0.org1.example.com
docker logs peer0.org2.example.com

# View installed chaincodes
peer lifecycle chaincode queryinstalled

# Check Docker networks
docker network ls
docker network inspect fabric_test
```

## 🏗️ Architecture

### Network Topology

```
┌─────────────────────────────────────────────────────────────┐
│                    Hyperledger Fabric Network               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────┐                  │
│  │   Orderer    │         │   Channel    │                  │
│  │              │◄────────┤  "mychannel" │                  │
│  └──────────────┘         └──────────────┘                  │
│         │                         │                         │
│         ├─────────────────────────┼──────────────────┐      │
│         │                         │                  │      │
│  ┌──────▼────────┐         ┌──────▼────────┐         │      │
│  │   Org1 Peer   │         │   Org2 Peer   │         │      │
│  │ peer0.org1    │         │ peer0.org2    │         │      │
│  └───────┬───────┘         └───────┬───────┘         │      │
│          │                         │                 │      │
│  ┌───────▼────────┐        ┌───────▼────────┐        │      │
│  │   Chaincode    │        │   Chaincode    │        │      │
│  │  arptracker    │        │  arptracker    │        │      │
│  │  (Container)   │        │  (Container)   │        │      │
│  └────────────────┘        └────────────────┘        │      │
│                                                      │      │
└──────────────────────────────────────────────────────┘      
```

### Chaincode Deployment (CCAAS)

This project uses **Chaincode as a Service (CCAAS)** model:

1. Chaincode runs in separate Docker containers
2. Peers connect to chaincode via gRPC
3. Provides better isolation and debugging capabilities
4. Suitable for cloud deployments (e.g., Azure VMs)

### Data Flow

```
Client Request
     │
     ▼
Peer CLI Command
     │
     ▼
Endorsing Peers (Org1 & Org2)
     │
     ▼
Execute Chaincode (CCAAS Containers)
     │
     ▼
Generate Proposal Response
     │
     ▼
Orderer (Consensus)
     │
     ▼
Commit to Ledger
     │
     ▼
Response to Client
```

## 🔄 Updating Chaincode

When you make changes to the chaincode:

```bash
# 1. Modify code
nano ~/fabric/arp-chaincode/arp-chaincode.go

# 2. Rebuild image
cd ~/fabric/arp-chaincode
docker build -t arptracker_ccaas_image:latest .

# 3. Restart containers
docker rm -f peer0org1_arptracker_ccaas peer0org2_arptracker_ccaas

docker run -d --name peer0org1_arptracker_ccaas \
  --network fabric_test \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -e CORE_CHAINCODE_ID_NAME=$PACKAGE_ID \
  arptracker_ccaas_image:latest

docker run -d --name peer0org2_arptracker_ccaas \
  --network fabric_test \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -e CORE_CHAINCODE_ID_NAME=$PACKAGE_ID \
  arptracker_ccaas_image:latest

# 4. Test changes
cd ~/fabric/fabric-samples/test-network
# Run your test commands
```

## 🧹 Cleanup

### Stop Everything

```bash
cd ~/fabric/fabric-samples/test-network

# Stop chaincode containers
docker rm -f peer0org1_arptracker_ccaas peer0org2_arptracker_ccaas

# Bring down the network
./network.sh down

# Remove Docker volumes (complete cleanup)
docker volume prune -f
```

## 🔐 Security Considerations

- **Permissioned Network**: Only authorized organizations can participate
- **TLS Enabled**: All communications are encrypted
- **Access Control**: Based on Fabric's MSP (Membership Service Provider)
- **Immutable Ledger**: Once recorded, ARP entries cannot be modified (only new entries added)
- **Audit Trail**: Complete history maintained for compliance

## 📚 Use Cases

1. **Network Security Monitoring**: Detect ARP spoofing attacks by tracking MAC address changes
2. **Compliance & Auditing**: Maintain immutable records of network topology changes
3. **Troubleshooting**: Historical ARP data helps diagnose network issues
4. **Multi-Site Networks**: Share ARP information across multiple locations securely
5. **IoT Networks**: Track device connectivity and detect unauthorized devices
