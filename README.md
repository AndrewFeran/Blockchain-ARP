# 🛡️ Blockchain-Based ARP Spoofing Detection System

A complete blockchain-based system for detecting and tracking ARP (Address Resolution Protocol) spoofing attacks using Hyperledger Fabric, with real-time event monitoring and a web dashboard.

## 🎯 Features

- **Real-time ARP Monitoring**: Track IP-to-MAC address mappings on immutable blockchain ledger
- **Spoofing Detection**: Automatically detect when an IP address changes its MAC address (ARP spoofing)
- **Event-Driven Architecture**: Real-time event notifications using Fabric Gateway SDK
- **Web Dashboard**: Live visualization of network events and spoofing alerts
- **Historical Tracking**: Complete audit trail of all ARP changes
- **Multi-Organization Support**: Built on Hyperledger Fabric's permissioned blockchain

---

## 📁 Project Structure

```
Blockchain-ARP/
├── chaincode/              # Smart contract (Go)
│   ├── arp-chaincode.go
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
│
├── dashboard/              # Web dashboard (Flask)
│   ├── app.py
│   ├── requirements.txt
│   └── templates/
│       └── index.html
│
├── event-listener/         # Event listener (Go)
│   ├── event-listener.go
│   ├── build-listener.sh
│   └── go.mod
│
└── scripts/                # Automation scripts
    ├── reset-and-start.sh
    ├── stop-all.sh
    └── README.md
```

---

## 🏗️ Architecture

```
┌─────────────────┐       Blockchain Events      ┌──────────────────┐
│  ARP Chaincode  │ ═══════════════════════════> │  Event Listener  │
│   (Fabric)      │   Real-time via Gateway SDK  │   (Go)           │
└─────────────────┘                              └─────────┬────────┘
                                                           │
                                                    HTTP POST
                                                           │
                                                           ▼
                                                  ┌─────────────────┐
                                                  │ Flask Dashboard │
                                                  │   (Web UI)      │
                                                  └─────────────────┘
```

### How It Works

1. **Chaincode** runs on blockchain, detects ARP changes, emits events
2. **Event Listener** subscribes to blockchain events in real-time
3. **Dashboard** displays events with color-coded alerts

---

## 🔧 Prerequisites

- **Ubuntu/Linux** VM (tested on Azure)
- **Docker** v20.10+
- **Docker Compose** v2.0+
- **Go** v1.21+
- **Python 3** v3.8+
- **Hyperledger Fabric** v2.5+

---

## 🚀 Quick Start

### 1. Install Hyperledger Fabric

```bash
mkdir -p ~/fabric
cd ~/fabric
curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh
chmod +x install-fabric.sh
./install-fabric.sh docker binary samples
```

### 2. Clone This Repository

```bash
cd ~/fabric
git clone <your-repo-url> arp-chaincode
cd arp-chaincode
```

### 3. Start Everything (One Command!)

```bash
cd scripts
chmod +x *.sh
./reset-and-start.sh
```

This will:
- Clean old network data
- Start Fabric network
- Build and deploy chaincode
- Set up environment
- Show you test commands

### 4. Start Dashboard (New Terminal)

```bash
cd ~/fabric/arp-chaincode/dashboard
pip3 install -r requirements.txt
python3 app.py
```

Dashboard runs at: **http://localhost:5000**

### 5. Start Event Listener (New Terminal)

```bash
cd ~/fabric/arp-chaincode/event-listener
go get github.com/hyperledger/fabric-gateway@v1.4.0
go mod tidy
go mod vendor
chmod +x build-listener.sh
./build-listener.sh
./event-listener
```

---

## 🧪 Testing the System

From the test-network directory:

```bash
cd ~/fabric/fabric-samples/test-network
```

### Record a New Device

```bash
peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" -c '{"function":"RecordARPEntry","Args":["192.168.1.100","AA:BB:CC:DD:EE:FF","eth0","laptop","dynamic","reachable","gateway1"]}'
```

**Expected:** `🆕 New device` in event listener and dashboard

### Trigger Spoofing Detection

Same IP, different MAC:

```bash
peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" -c '{"function":"RecordARPEntry","Args":["192.168.1.100","11:22:33:44:55:66","eth0","laptop","dynamic","reachable","gateway1"]}'
```

**Expected:** `🚨 SPOOFING DETECTED!` alert in red

### Query All Entries

```bash
peer chaincode query -C mychannel -n arptracker -c '{"function":"GetAllARPEntries","Args":[]}'
```

---

## 📊 Chaincode Functions

### RecordARPEntry
Records ARP entry to blockchain and emits detection event

**Args:** `[ipAddress, macAddress, interface, hostname, entryType, state, recordedBy]`

### GetCurrentARPEntry
Get current ARP entry for an IP

**Args:** `[ipAddress]`

### GetARPHistory
Get complete history of changes for an IP

**Args:** `[ipAddress]`

### GetAllARPEntries
Get all current ARP entries

**Args:** `[]`

### QueryARPByMAC
Find all IPs associated with a MAC address

**Args:** `[macAddress]`

### DetectMACChange
Check if MAC changed for an IP

**Args:** `[ipAddress, currentMAC]`

---

## 🌐 Dashboard Features

- **Real-time Statistics**: Total events, spoofing detections, new devices
- **Event Feed**: Live stream of ARP events with color coding
- **Auto-refresh**: Updates every 2 seconds
- **Event Types**:
  - 🆕 **New** - First time seeing this IP
  - 🚨 **Spoofing** - MAC changed (potential attack!)
  - ✅ **Match** - Valid update (MAC unchanged)

### API Endpoints

- `GET /` - Dashboard UI
- `POST /api/event` - Receive events from listener
- `GET /api/events?limit=N` - Get recent events (JSON)
- `GET /api/stats` - Get statistics (JSON)

---

## 🛠️ Troubleshooting

### Network Issues

```bash
# Check what's running
docker ps

# View logs
docker logs peer0org1_arptracker_ccaas
docker logs peer0.org1.example.com

# Restart everything
cd ~/fabric/arp-chaincode/scripts
./stop-all.sh
./reset-and-start.sh
```

### Event Listener Not Receiving Events

1. Check chaincode is running: `docker ps | grep arptracker`
2. Verify Flask is running: `curl http://localhost:5000`
3. Check listener logs for connection errors
4. Ensure you're in correct directory when invoking chaincode

### Dashboard Not Loading

```bash
# Check Flask process
ps aux | grep app.py

# Test Flask API
curl http://localhost:5000/api/stats

# Restart Flask
cd ~/fabric/arp-chaincode/dashboard
python3 app.py
```

---

## 🔐 Security Considerations

- **Permissioned Network**: Only authorized organizations participate
- **TLS Enabled**: All communications encrypted
- **Immutable Ledger**: Records cannot be tampered with
- **Event-Driven**: No polling - instant notification of changes
- **Audit Trail**: Complete history for compliance

---

## 🔄 Stopping the System

```bash
# Stop everything
cd ~/fabric/arp-chaincode/scripts
./stop-all.sh

# This stops:
# - Fabric network
# - All peer and orderer containers
# - Chaincode containers
```

Then manually stop Flask (Ctrl+C) and event listener (Ctrl+C).

---

## 📚 Use Cases

1. **Network Security Monitoring**: Real-time detection of ARP spoofing attacks
2. **Compliance & Auditing**: Immutable records of network changes
3. **Incident Response**: Complete history for forensic analysis
4. **Multi-Site Networks**: Share ARP data securely across locations
5. **IoT Security**: Track device connectivity and detect rogue devices

---

## 🎓 Learn More

- [Hyperledger Fabric Docs](https://hyperledger-fabric.readthedocs.io/)
- [ARP Spoofing Explained](https://en.wikipedia.org/wiki/ARP_spoofing)
- [Fabric Gateway SDK](https://hyperledger.github.io/fabric-gateway/)

---

**Built with Hyperledger Fabric** | **Blockchain-secured Network Monitoring**