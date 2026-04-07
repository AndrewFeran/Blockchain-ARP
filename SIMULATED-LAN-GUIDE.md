# 🌐 Simulated LAN with Blockchain-Secured ARP

This guide explains how to run a **simulated LAN environment** where multiple containerized nodes synchronize their ARP tables via a blockchain, preventing ARP spoofing attacks.

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│           Simulated LAN Network (10.5.0.0/24)                │
│                                                               │
│  ┌──────────────┐                                            │
│  │   Router     │  Monitors ALL ARP traffic                 │
│  │  10.5.0.1    │  Writes to Blockchain                     │
│  │  (Authority) │                                            │
│  └──────┬───────┘                                            │
│         │                                                     │
│         │ Captures ARP packets                               │
│         ▼                                                     │
│  ┌─────────────────────────────┐                            │
│  │  Hyperledger Fabric         │                            │
│  │  Blockchain Network         │◄───────────────┐           │
│  │  (External - Host)          │                │           │
│  └─────────────┬───────────────┘                │           │
│                │                                 │           │
│         Events │                          Reads  │           │
│                ▼                                 │           │
│    ┌──────┬──────────┬──────────┐               │           │
│    │      │          │          │               │           │
│ ┌──▼──┐ ┌▼────┐  ┌──▼──┐    ┌──▼──┐           │           │
│ │Node1│ │Node2│  │Node3│    │Attkr│           │           │
│ │.10  │ │.11  │  │.12  │    │.99  │───────────┘           │
│ └─────┘ └─────┘  └─────┘    └─────┘                        │
│    │       │         │          │                            │
│    └───────┴─────────┴──────────┘                           │
│     All sync ARP cache from blockchain                       │
└─────────────────────────────────────────────────────────────┘
```

### Components

1. **Router Container** (`10.5.0.1`)
   - Captures all ARP packets using `gopacket`
   - Writes every ARP mapping to blockchain
   - Acts as the network gateway/authority

2. **Node Containers** (`10.5.0.10-12`)
   - Subscribe to blockchain ARP events
   - Automatically populate their ARP cache from blockchain
   - Reject spoofed entries detected by blockchain

3. **Attacker Container** (`10.5.0.99`) - Optional
   - Used for testing ARP spoofing attacks
   - Demonstrates blockchain protection

4. **Dashboard** (`localhost:5000`)
   - Real-time visualization of ARP events
   - Color-coded alerts for spoofing detection

---

## 🚀 Quick Start

### Prerequisites

1. **Hyperledger Fabric test network running**
2. **ARP chaincode deployed**
3. **Docker and Docker Compose installed**

### Step 1: Start Fabric Network (if not running)

```bash
cd ~/fabric/fabric-samples/test-network
./network.sh up createChannel -c mychannel
```

### Step 2: Deploy ARP Chaincode (if not deployed)

```bash
cd ~/fabric/fabric-samples/test-network
./network.sh deployCC -ccn arptracker -ccp ~/fabric/arp-chaincode/chaincode -ccl go
```

### Step 3: Start Simulated LAN

```bash
cd ~/fabric/arp-chaincode
chmod +x *.sh
./start-simulated-lan.sh
```

This will:
- Build all Docker images
- Start router, 3 nodes, and dashboard
- Connect them to blockchain

### Step 4: View Dashboard

Open browser: **http://localhost:5000**

You'll see real-time ARP activity!

---

## 🧪 Testing ARP Spoofing Detection

### Option 1: Automated Demo

```bash
./demo-spoofing-attack.sh
```

This interactive script will:
1. Establish legitimate ARP mappings
2. Launch simulated ARP spoofing attack
3. Show blockchain detection in action

### Option 2: Manual Testing

**Start attacker container:**
```bash
docker-compose --profile testing up -d attacker
```

**Generate legitimate traffic:**
```bash
docker exec lan-node-1 ping -c 3 10.5.0.11
```

Check dashboard - you should see "New Device" events.

**Launch attack:**
```bash
docker exec lan-attacker /app/spoof-attack.sh 10.5.0.10 aa:bb:cc:dd:ee:99
```

**Result:** Dashboard shows 🚨 **SPOOFING DETECTED!** in red

---

## 📊 Monitoring

### View Container Logs

**Router (ARP capture):**
```bash
docker logs -f blockchain-router
```

**Node 1 (ARP sync):**
```bash
docker logs -f lan-node-1
```

**All services:**
```bash
docker-compose logs -f
```

### View ARP Tables

**Check router's view:**
```bash
docker exec blockchain-router ip neigh show
```

**Check node's synchronized table:**
```bash
docker exec lan-node-1 ip neigh show
```

### Exec into Containers

```bash
docker exec -it blockchain-router /bin/bash
docker exec -it lan-node-1 /bin/bash
```

---

## 🔬 How It Works

### 1. ARP Traffic Generation

Nodes automatically ping each other every 30 seconds, generating real ARP traffic:

```
Node1 → "Who has 10.5.0.11?"
Node2 → "10.5.0.11 is at aa:bb:cc:dd:ee:02"
```

### 2. Router Captures

Router sees the ARP reply and writes to blockchain:

```go
contract.SubmitTransaction("RecordARPEntry",
    "10.5.0.11",
    "aa:bb:cc:dd:ee:02",
    "eth0", "node2", "dynamic", "reachable", "router")
```

### 3. Chaincode Processes

[arp-chaincode.go](chaincode/arp-chaincode.go:100) checks:
- Is this IP new? → Event: `"new"`
- Did MAC change? → Event: `"spoofing"`
- Same MAC? → Event: `"match"`

### 4. Nodes Receive Events

All nodes subscribe to blockchain events:

```go
events := network.ChaincodeEvents(ctx, "arptracker")
for event := range events {
    if event.EventType == "spoofing" {
        // REJECT the fake entry!
    } else {
        // Add to ARP cache
        ip neigh add <IP> lladdr <MAC> dev eth0 nud permanent
    }
}
```

### 5. Protection

When spoofing is detected:
- ✅ Dashboard shows RED ALERT
- ✅ Nodes reject fake MAC
- ✅ Legitimate mapping preserved
- ✅ Complete audit trail

---

## 🎯 Use Cases Demonstrated

1. **Distributed ARP Cache Sync**
   - All nodes have identical, synchronized ARP tables
   - Source of truth is blockchain

2. **Spoofing Detection**
   - Automatic detection when IP changes MAC
   - Instant alerting to all participants

3. **Attack Prevention**
   - Nodes reject poisoned ARP entries
   - Blockchain prevents cache poisoning

4. **Audit Trail**
   - Complete history of all ARP changes
   - Immutable record for forensics

---

## 🛠️ Troubleshooting

### Router not capturing ARP

**Check privileges:**
```bash
docker exec blockchain-router ip link show eth0
```

Router needs `NET_ADMIN` and `NET_RAW` capabilities (already set in docker-compose.yml).

### Nodes not connecting to blockchain

**Check peer connectivity:**
```bash
docker exec lan-node-1 nc -zv host.docker.internal 9051
```

If fails, check `extra_hosts` in docker-compose.yml.

### No ARP traffic

**Manually trigger:**
```bash
docker exec lan-node-1 ping -c 3 10.5.0.11
```

### Dashboard not receiving events

**Check Flask:**
```bash
curl http://localhost:5000/api/stats
```

**Check router connection:**
```bash
docker logs blockchain-router | grep "Connected to blockchain"
```

---

## 🧹 Cleanup

**Stop all containers:**
```bash
./stop-simulated-lan.sh
```

Or manually:
```bash
docker-compose down
```

**Remove images:**
```bash
docker-compose down --rmi all
```

---

## 📁 File Structure

```
Blockchain-ARP/
├── docker-compose.yml          # Orchestrates all containers
│
├── router/                     # Router service (ARP authority)
│   ├── Dockerfile
│   ├── router-monitor.go       # Captures ARP, writes to blockchain
│   ├── start-router.sh
│   └── capture-arp.sh          # Helper for debugging
│
├── node/                       # Node service (LAN hosts)
│   ├── Dockerfile
│   ├── node-sync.go            # Reads blockchain, syncs ARP cache
│   ├── start-node.sh
│   ├── arp-manager.sh          # Manual ARP management
│   └── spoof-attack.sh         # Attack simulation
│
├── dashboard/                  # Web UI
│   ├── Dockerfile
│   └── app.py
│
├── start-simulated-lan.sh      # Start everything
├── stop-simulated-lan.sh       # Stop everything
├── demo-spoofing-attack.sh     # Interactive attack demo
│
└── SIMULATED-LAN-GUIDE.md      # This file
```

---

## 🎓 Learning Outcomes

This simulated LAN demonstrates:

1. **Real network behavior in containers**
   - Docker bridge networks use actual Layer 2 protocols
   - ARP traffic is genuine, not mocked

2. **Blockchain for network security**
   - Distributed consensus prevents tampering
   - Immutable audit trail for compliance

3. **Event-driven architecture**
   - Real-time synchronization via chaincode events
   - Scalable to many nodes

4. **Practical ARP spoofing prevention**
   - Detection happens automatically
   - Protection requires no user intervention

---

## 🔗 Related Documentation

- [Main README](README.md) - Original project documentation
- [Chaincode Source](chaincode/arp-chaincode.go) - Smart contract logic
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Hyperledger Fabric Gateway SDK](https://hyperledger.github.io/fabric-gateway/)

---

**Built with Hyperledger Fabric** | **Blockchain-Secured Network Simulation**
