# 🚀 ARP Detection System - Quick Start Scripts

Two simple scripts to manage your blockchain-based ARP detection system.

## 📋 Available Scripts

### `reset-and-start.sh` - Complete Setup ⭐
**One command does everything!**

Automatically:
- Deep cleans old network data
- Starts Fabric network
- Creates channel
- Builds and deploys chaincode
- Sets up environment
- Shows test commands

```bash
cd ~/fabric/arp-chaincode/scripts
chmod +x reset-and-start.sh
./reset-and-start.sh
```

### `stop-all.sh` - Shutdown

Stops the Fabric network and cleans up containers.

```bash
./stop-all.sh
```

---

## 🎯 Common Workflows

### First Time Setup

```bash
cd ~/fabric/arp-chaincode/scripts
chmod +x *.sh
./reset-and-start.sh
```

### Daily Use

```bash
# Start everything
./reset-and-start.sh

# When done
./stop-all.sh
```

### When You Have Issues

```bash
# Just run the reset script - it handles everything
./reset-and-start.sh
```

---

## 🧪 Testing Commands

After running `reset-and-start.sh`, the script outputs ready-to-use test commands.

Or copy these:

**Record a new device:**
```bash
peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" -c '{"function":"RecordARPEntry","Args":["192.168.1.100","AA:BB:CC:DD:EE:FF","eth0","laptop","dynamic","reachable","gateway1"]}'
```

**Test spoofing (same IP, different MAC):**
```bash
peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" -C mychannel -n arptracker --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" -c '{"function":"RecordARPEntry","Args":["192.168.1.100","11:22:33:44:55:66","eth0","laptop","dynamic","reachable","gateway1"]}'
```

**Query all entries:**
```bash
peer chaincode query -C mychannel -n arptracker -c '{"function":"GetAllARPEntries","Args":[]}'
```

---

## 📊 Start Dashboard & Event Listener

After blockchain is running:

```bash
# Terminal 1: Flask Dashboard
cd ~/fabric/arp-chaincode/dashboard
python3 app.py

# Terminal 2: Build and run REAL event listener (Go)
cd ~/fabric/arp-chaincode
chmod +x build-listener.sh
./build-listener.sh
./event-listener

# Access at: http://localhost:5000